package services

import (
	"Backend/internal/companyfetch"
	"Backend/internal/config"
	"Backend/internal/models"
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"unicode"
)

var companyValidationCacheTTL = 30 * time.Minute

const companyValidationMaxTok = 200

func init() {
	companyValidationCacheTTL = time.Duration(config.ValidationCacheTTLMinutes()) * time.Minute
}

// companyLookup は企業実在確認に必要な最小の参照インターフェース。
type companyLookup interface {
	FindByName(name string) (*models.Company, error)
	FindAllActiveNames(q string) ([]models.CompanyName, error)
}

// CompanyValidationResult は企業実在確認の結果。
type CompanyValidationResult struct {
	Exists         bool     `json:"exists"`
	CanonicalName  string   `json:"canonical_name"`
	CompanyID      *uint    `json:"company_id,omitempty"`
	EvidenceURLs   []string `json:"evidence_urls"`
	Source         string   `json:"source"` // db | web_search | cache
	Confidence     string   `json:"confidence"`
	Description    string   `json:"description,omitempty"`
	Query          string   `json:"query"`
	FromCache      bool     `json:"from_cache"`
	TokensEstimate int      `json:"tokens_estimate,omitempty"`
}

type companyValidationCacheEntry struct {
	result    CompanyValidationResult
	expiresAt time.Time
}

// CompanyValidationService は DB 優先・必要時のみ軽量 WebSearch で企業の実在を確認する。
type CompanyValidationService struct {
	companyRepo  companyLookup
	openaiClient *openai.Client
	budget       companyfetch.SearchBudget
	flight       *CompanySearchFlight
	mu           sync.RWMutex
	cache        map[string]companyValidationCacheEntry
}

func NewCompanyValidationService(companyRepo companyLookup, client *openai.Client) *CompanyValidationService {
	s := &CompanyValidationService{
		companyRepo:  companyRepo,
		openaiClient: client,
		cache:        make(map[string]companyValidationCacheEntry),
	}
	go s.purgeLoop()
	return s
}

// SetSearchBudget は月次 Search 予算ガードを注入する。
func (s *CompanyValidationService) SetSearchBudget(budget companyfetch.SearchBudget) {
	if s != nil {
		s.budget = budget
	}
}

// SetSearchFlight は企業キー単位の singleflight を注入する。
func (s *CompanyValidationService) SetSearchFlight(flight *CompanySearchFlight) {
	if s != nil {
		s.flight = flight
	}
}

// Validate は企業名の実在を確認する。
// 優先順: メモリキャッシュ → DB 完全一致/部分一致 → OpenAI WebSearch（1クエリ・短文）。
func (s *CompanyValidationService) Validate(ctx context.Context, query string) (*CompanyValidationResult, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, &ValidationError{Message: "企業名を入力してください"}
	}
	key := normalizeCompanyKey(q)

	if cached, ok := s.getCache(key); ok {
		cached.FromCache = true
		cached.Source = "cache"
		cached.Query = q
		return &cached, nil
	}

	if s.companyRepo != nil {
		if company, err := s.companyRepo.FindByName(q); err == nil && company != nil {
			id := company.ID
			result := CompanyValidationResult{
				Exists:        true,
				CanonicalName: company.Name,
				CompanyID:     &id,
				EvidenceURLs:  nonEmptyURLs(company.WebsiteURL),
				Source:        "db",
				Confidence:    "high",
				Description:   truncateRunes(company.Description, 80),
				Query:         q,
			}
			s.putCache(key, result)
			return &result, nil
		}

		if names, err := s.companyRepo.FindAllActiveNames(q); err == nil {
			for _, n := range names {
				if normalizeCompanyKey(n.Name) == key {
					id := n.ID
					result := CompanyValidationResult{
						Exists:        true,
						CanonicalName: n.Name,
						CompanyID:     &id,
						Source:        "db",
						Confidence:    "high",
						Query:         q,
					}
					s.putCache(key, result)
					return &result, nil
				}
			}
			// 部分一致が1件だけなら DB 確定（トークン 0）
			if len(names) == 1 {
				id := names[0].ID
				result := CompanyValidationResult{
					Exists:        true,
					CanonicalName: names[0].Name,
					CompanyID:     &id,
					Source:        "db",
					Confidence:    "medium",
					Query:         q,
				}
				s.putCache(key, result)
				return &result, nil
			}
		}
	}

	result, err := s.validateWithWebSearch(ctx, q)
	if err != nil {
		return nil, err
	}
	s.putCache(key, *result)
	return result, nil
}

// SearchCandidates は DB 候補を返し、必要なら WebSearch で実在候補を補完する。
func (s *CompanyValidationService) SearchCandidates(ctx context.Context, query string, includeWebSearch bool) ([]CompanyValidationResult, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, &ValidationError{Message: "検索キーワードを入力してください"}
	}

	var out []CompanyValidationResult
	seen := map[string]struct{}{}

	if s.companyRepo != nil {
		if names, err := s.companyRepo.FindAllActiveNames(q); err == nil {
			limit := 5
			if len(names) < limit {
				limit = len(names)
			}
			for i := 0; i < limit; i++ {
				n := names[i]
				id := n.ID
				key := normalizeCompanyKey(n.Name)
				seen[key] = struct{}{}
				out = append(out, CompanyValidationResult{
					Exists:        true,
					CanonicalName: n.Name,
					CompanyID:     &id,
					Source:        "db",
					Confidence:    "high",
					Query:         q,
				})
			}
		}
	}

	if !includeWebSearch || len(out) > 0 {
		return out, nil
	}

	validated, err := s.Validate(ctx, q)
	if err != nil {
		// WEB 補完失敗時も DB 結果（空）を返し、UI でメッセージ表示できるようにする
		return out, nil
	}
	if validated.Exists {
		key := normalizeCompanyKey(validated.CanonicalName)
		if _, ok := seen[key]; !ok {
			out = append(out, *validated)
		}
	}
	return out, nil
}

func (s *CompanyValidationService) validateWithWebSearch(ctx context.Context, query string) (*CompanyValidationResult, error) {
	if s.openaiClient == nil {
		return &CompanyValidationResult{
			Exists:        false,
			CanonicalName: "",
			Source:        "web_search",
			Confidence:    "low",
			Query:         query,
		}, nil
	}

	run := func() (*CompanyValidationResult, error) {
		if s.budget != nil {
			if err := s.budget.AllowSearch(); err != nil {
				if errors.Is(err, companyfetch.ErrSearchBudgetExceeded) {
					return &CompanyValidationResult{
						Exists:        false,
						CanonicalName: "",
						Source:        "cache",
						Confidence:    "low",
						Query:         query,
						Description:   "月次の企業Search上限に達したため、キャッシュのみで確認しています。しばらくしてから再度お試しください",
					}, nil
				}
				return nil, err
			}
		}

		prompt := fmt.Sprintf(`日本の実在企業かどうかだけを判定してください。推測で企業を作らないでください。
検索対象: 「%s」

次のJSONオブジェクトのみを返してください（説明文不要）:
{"exists":true/false,"canonical_name":"正式な企業名（存在する場合）","evidence_urls":["公式サイト等のURL"],"confidence":"high|medium|low","description":"事業の1行説明"}

exists=false の場合は canonical_name を空文字、evidence_urls を空配列にしてください。`, query)

		text, err := s.openaiClient.WebSearchJSON(ctx, prompt, companyValidationMaxTok)
		if err != nil {
			log.Printf("[CompanyValidation] web search failed query=%q err=%v", query, err)
			return &CompanyValidationResult{
				Exists:        false,
				CanonicalName: "",
				Source:        "web_search",
				Confidence:    "low",
				Query:         query,
				Description:   "WEB検索による実在確認に失敗しました。企業名を変えるか、しばらくしてから再度お試しください",
			}, nil
		}

		parsed, parseErr := parseValidationJSON(text)
		if parseErr != nil {
			log.Printf("[CompanyValidation] parse failed query=%q raw=%q err=%v", query, truncateRunes(text, 200), parseErr)
			return &CompanyValidationResult{
				Exists:     false,
				Source:     "web_search",
				Confidence: "low",
				Query:      query,
			}, nil
		}

		parsed.Source = "web_search"
		parsed.Query = query
		parsed.TokensEstimate = companyValidationMaxTok
		if parsed.Exists && strings.TrimSpace(parsed.CanonicalName) == "" {
			parsed.CanonicalName = query
		}
		if parsed.EvidenceURLs == nil {
			parsed.EvidenceURLs = []string{}
		}
		log.Printf("[CompanyValidation] web_search query=%q exists=%v canonical=%q confidence=%s",
			query, parsed.Exists, parsed.CanonicalName, parsed.Confidence)
		return parsed, nil
	}

	if s.flight == nil {
		return run()
	}
	v, err := s.flight.Do("validate", normalizeCompanyKey(query), func() (any, error) {
		return run()
	})
	if err != nil {
		return nil, err
	}
	if v == nil {
		return &CompanyValidationResult{Exists: false, Source: "web_search", Confidence: "low", Query: query}, nil
	}
	result, _ := v.(*CompanyValidationResult)
	return result, nil
}

func parseValidationJSON(text string) (*CompanyValidationResult, error) {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no json object")
	}
	var parsed CompanyValidationResult
	if err := json.Unmarshal([]byte(text[start:end+1]), &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func (s *CompanyValidationService) getCache(key string) (CompanyValidationResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.cache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return CompanyValidationResult{}, false
	}
	return entry.result, true
}

func (s *CompanyValidationService) purgeLoop() {
	ticker := time.NewTicker(companyValidationCacheTTL)
	defer ticker.Stop()
	for range ticker.C {
		s.purgeExpired()
	}
}

func (s *CompanyValidationService) purgeExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, v := range s.cache {
		if now.After(v.expiresAt) {
			delete(s.cache, k)
		}
	}
}

func (s *CompanyValidationService) putCache(key string, result CompanyValidationResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := result
	cp.FromCache = false
	s.cache[key] = companyValidationCacheEntry{
		result:    cp,
		expiresAt: time.Now().Add(companyValidationCacheTTL),
	}
}

func normalizeCompanyKey(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if unicode.IsSpace(r) || r == '　' {
			continue
		}
		b.WriteRune(r)
	}
	s := b.String()
	s = strings.ReplaceAll(s, "株式会社", "")
	s = strings.ReplaceAll(s, "(株)", "")
	s = strings.ReplaceAll(s, "（株）", "")
	return s
}

func nonEmptyURLs(url string) []string {
	url = strings.TrimSpace(url)
	if url == "" {
		return []string{}
	}
	return []string{url}
}

func truncateRunes(s string, max int) string {
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	return string(rs[:max]) + "…"
}
