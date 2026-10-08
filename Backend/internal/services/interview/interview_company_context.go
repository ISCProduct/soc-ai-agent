package interview

import (
	"Backend/internal/models"
	"Backend/internal/services/company"
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

const companyReadingCacheTTL = 24 * time.Hour
const companyReadingCacheMaxEntries = 1024

type companyReadingCacheEntry struct {
	value     string
	expiresAt time.Time
}

type companyReadingCache struct {
	mu      sync.Mutex
	entries map[string]companyReadingCacheEntry
}

func (c *companyReadingCache) load(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return "", false
	}
	if !time.Now().Before(entry.expiresAt) {
		delete(c.entries, key)
		return "", false
	}
	return entry.value, true
}

func (c *companyReadingCache) store(key, value string) {
	if key == "" || strings.TrimSpace(value) == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for cachedKey, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, cachedKey)
		}
	}
	if c.entries == nil {
		c.entries = make(map[string]companyReadingCacheEntry)
	}
	if _, exists := c.entries[key]; !exists && len(c.entries) >= companyReadingCacheMaxEntries {
		var oldestKey string
		var oldestExpiry time.Time
		for cachedKey, entry := range c.entries {
			if oldestKey == "" || entry.expiresAt.Before(oldestExpiry) {
				oldestKey, oldestExpiry = cachedKey, entry.expiresAt
			}
		}
		delete(c.entries, oldestKey)
	}
	c.entries[key] = companyReadingCacheEntry{
		value:     strings.TrimSpace(value),
		expiresAt: now.Add(companyReadingCacheTTL),
	}
}

// resolveCompanyInfo は共有企業情報を優先し、無ければクライアント文面を使う。
// Search/LLM 調査はしない。companyType（general/sier）ではゲートしない。
func (s *InterviewService) resolveCompanyInfo(companyID uint, companyName, clientInfo string) string {
	brief := ""
	if companyID > 0 || strings.TrimSpace(companyName) != "" {
		brief = s.lookupCompanyProfile(companyID, companyName)
	}
	if brief != "" {
		return brief
	}
	return strings.TrimSpace(clientInfo)
}

// lookupCompanyProfile は共有キャッシュ（companies）から企業スナップショットを読む。
// Search / LLM による企業調査は行わない。未登録・未整備なら空文字。
func (s *InterviewService) lookupCompanyProfile(companyID uint, companyName string) string {
	comp := s.findCompany(companyID, companyName)
	if comp == nil {
		return ""
	}
	var profile *models.CompanyWeightProfile
	if s.companyRepo != nil {
		if p, err := s.companyRepo.GetWeightProfile(comp.ID, nil); err == nil {
			profile = p
		}
	}
	return company.BuildCompanyBrief(comp, profile)
}

func (s *InterviewService) findCompany(companyID uint, companyName string) *models.Company {
	if s.companyRepo == nil {
		return nil
	}
	if companyID > 0 {
		if c, err := s.companyRepo.FindByID(companyID); err == nil && c != nil {
			return c
		}
	}
	name := strings.TrimSpace(companyName)
	if name == "" {
		return nil
	}
	c, err := s.companyRepo.FindByName(name)
	if err != nil || c == nil {
		return nil
	}
	return c
}

// resolveCompanyID は company_id=0（WEB検索・手入力）でも企業名が DB と一致すれば ID を解決する。
// 解決できない場合は 0 のまま（カスタム質問・深掘りは無効）。
func (s *InterviewService) resolveCompanyID(companyID uint, companyName string) uint {
	if company := s.findCompany(companyID, companyName); company != nil {
		return company.ID
	}
	return 0
}

// sttHintCompany は音声認識の補助語に使える企業名・読みを DB から返す。
//
// 解決できなければ両方空にする。補助語は「御社」＋ companyInfo の技術用語だけになり、
// #1603 で測った固有名詞の改善は DB 登録済みの企業で保たれる。
// クライアント直値を補助語へ入れないのが目的なので、ここで LLM 補完
// （lookupCompanyReading）は使わない。STT はターンの先頭にあり、
// ここに LLM 呼び出しを足すと応答遅延がそのまま増える。
func (s *InterviewService) sttHintCompany(companyID uint, companyName string) (string, string) {
	c := s.findCompany(companyID, companyName)
	if c == nil {
		return "", ""
	}
	return strings.TrimSpace(c.Name), strings.TrimSpace(c.NameReading)
}

// resolveCompanyReading は共有DBの NameReading を優先し、無ければモデル知識で補完する。
func (s *InterviewService) resolveCompanyReading(ctx context.Context, companyID uint, companyName string) string {
	if company := s.findCompany(companyID, companyName); company != nil {
		if reading := strings.TrimSpace(company.NameReading); reading != "" {
			return reading
		}
	}
	cacheKey := strings.TrimSpace(companyName)
	if cacheKey == "" {
		return ""
	}
	return s.cachedCompanyReading(ctx, cacheKey, companyName, s.lookupCompanyReading)
}

func (s *InterviewService) cachedCompanyReading(
	ctx context.Context,
	cacheKey string,
	companyName string,
	lookupReading func(context.Context, string) (string, error),
) string {
	if cached, ok := s.companyReadingCache.load(cacheKey); ok {
		return cached
	}

	lookup := func() (any, error) {
		if cached, ok := s.companyReadingCache.load(cacheKey); ok {
			return cached, nil
		}
		reading, err := lookupReading(ctx, companyName)
		if err != nil {
			return "", err
		}
		s.companyReadingCache.store(cacheKey, reading)
		return strings.TrimSpace(reading), nil
	}

	value, err, shared := s.companyReadingFlight.Do(cacheKey, lookup)
	if errors.Is(err, context.Canceled) && shared && ctx.Err() == nil {
		// 共有元のリクエストがキャンセルされた場合だけ、待機側の有効なctxで再試行する。
		// 再びsingleflightを通し、複数の待機リクエストが一斉に外部APIを呼ばないようにする。
		retryCh := s.companyReadingFlight.DoChan(cacheKey, lookup)
		select {
		case retryResult := <-retryCh:
			value, err = retryResult.Val, retryResult.Err
		case <-ctx.Done():
			return ""
		}
	}
	if err != nil {
		return ""
	}
	reading, _ := value.(string)
	return strings.TrimSpace(reading)
}
