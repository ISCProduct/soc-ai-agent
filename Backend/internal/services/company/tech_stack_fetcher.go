package company

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"Backend/internal/companyfields"
	"Backend/internal/models"
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TechStackResult は技術スタック取得結果。
type TechStackResult struct {
	TechStack        []string `json:"tech_stack"`
	InfraStack       []string `json:"infra_stack"`
	CicdTools        []string `json:"cicd_tools"`
	DevelopmentStyle string   `json:"development_style"`
	Source           string   `json:"source,omitempty"`
	SourceURL        string   `json:"source_url,omitempty"`
	ModelUsed        string   `json:"model_used,omitempty"`
	Confidence       string   `json:"confidence,omitempty"`
	FromCache        bool     `json:"from_cache,omitempty"`
	SkipReason       string   `json:"skip_reason,omitempty"`
}

// TechStackFetcher は安価な AI Search（mini-search→Parse）で技術スタックを取得する。
type TechStackFetcher struct {
	repo   repository.CompanyRepository
	llm    *companyfetch.LLM
	flight *CompanySearchFlight
}

func NewTechStackFetcher(repo repository.CompanyRepository, client *openai.Client) *TechStackFetcher {
	return &TechStackFetcher{repo: repo, llm: companyfetch.NewLLM(client)}
}

// SetSearchBudget は月次 Search 予算ガードを注入する。
func (f *TechStackFetcher) SetSearchBudget(budget companyfetch.SearchBudget) {
	if f == nil {
		return
	}
	if f.llm == nil {
		f.llm = &companyfetch.LLM{}
	}
	f.llm.Budget = budget
}

// SetSearchFlight は企業キー単位の singleflight を注入する。
func (f *TechStackFetcher) SetSearchFlight(flight *CompanySearchFlight) {
	if f != nil {
		f.flight = flight
	}
}

const techStackJSONSchemaIT = `{
  "tech_stack": ["言語・フレームワーク名（例: Go, React, TypeScript）"],
  "infra_stack": ["インフラ名（例: AWS, GCP, Azure, オンプレ）"],
  "cicd_tools": ["CI/CDツール名（例: GitHub Actions, Jenkins, CircleCI）"],
  "development_style": "開発手法（例: スクラム, ウォーターフォール, カンバン）"
}`

const techStackJSONSchemaManufacturing = `{
  "tech_stack": ["主要技術・製品技術（例: 精密加工, 車載センサー, CAD/CAM）"],
  "infra_stack": ["生産設備・拠点（例: 国内工場, SMTライン, クリーンルーム）"],
  "cicd_tools": [],
  "development_style": "生産・開発の進め方（例: セル生産, ライン生産, 受注生産）"
}`

// FetchAndSave は技術スタックを取得して DB を更新する。
func (f *TechStackFetcher) FetchAndSave(ctx context.Context, companyID uint, forceRefresh bool) (*TechStackResult, error) {
	company, err := f.repo.FindByID(companyID)
	if err != nil {
		return nil, fmt.Errorf("company not found: %w", err)
	}

	// TTL内でも技術データが空なら再取得する（スタンプだけ進んだ残骸対策）
	if !forceRefresh && companyfetch.IsFresh(company.TechFetchedAt, companyfetch.TTLTech) &&
		companyfetch.HasTechDataForIndustry(company.Industry, company.TechStack, company.InfraStack, company.CicdTools, company.DevelopmentStyle) {
		result := techResultFromCompany(company)
		result.FromCache = true
		result.SkipReason = "ttl"
		return result, nil
	}

	run := func() (any, error) {
		return f.Acquire(ctx, company.Name, company.WebsiteURL, company.Industry)
	}
	var result *TechStackResult
	if f.flight != nil {
		v, ferr := f.flight.Do("tech", normalizeCompanyKey(company.Name), run)
		err = ferr
		if v != nil {
			result, _ = v.(*TechStackResult)
		}
	} else {
		result, err = f.Acquire(ctx, company.Name, company.WebsiteURL, company.Industry)
	}
	if err != nil {
		if errors.Is(err, companyfetch.ErrSearchBudgetExceeded) {
			cached := techResultFromCompany(company)
			cached.FromCache = true
			cached.SkipReason = "budget"
			return cached, nil
		}
		return nil, err
	}

	if len(result.TechStack) > 0 {
		if b, err := json.Marshal(result.TechStack); err == nil {
			company.TechStack = string(b)
		}
	}
	if len(result.InfraStack) > 0 {
		if b, err := json.Marshal(result.InfraStack); err == nil {
			company.InfraStack = string(b)
		}
	}
	if len(result.CicdTools) > 0 {
		if b, err := json.Marshal(result.CicdTools); err == nil {
			company.CicdTools = string(b)
		}
	}
	if result.DevelopmentStyle != "" {
		company.DevelopmentStyle = result.DevelopmentStyle
	}

	now := time.Now()
	// 空のまま TechFetchedAt だけ進むと不足埋めが止まるため、中身があるときのみスタンプ
	if companyfetch.HasTechDataForIndustry(company.Industry, company.TechStack, company.InfraStack, company.CicdTools, company.DevelopmentStyle) {
		company.TechFetchedAt = &now
	}
	company.SourceFetchedAt = &now
	if result.Source != "" {
		company.SourceType = result.Source
	}
	company.SourceURL = result.SourceURL
	company.LastModelUsed = result.ModelUsed
	company.LastFetchConfidence = result.Confidence

	if err := f.repo.Update(company); err != nil {
		return nil, fmt.Errorf("failed to update company: %w", err)
	}
	return result, nil
}

// techAcquirePrompts は業種に応じた抽出スキーマ・システムプロンプト・検索プロンプト・表示名を返す。
func techAcquirePrompts(industry, companyName, websiteURL string) (schema, systemPrompt, searchPrompt, domainLabel string) {
	profile := companyfields.Resolve(industry)
	siteHint := websiteURLOrUnknown(websiteURL)
	if profile.ID == companyfields.ProfileManufacturing {
		return techStackJSONSchemaManufacturing,
			`あなたは製造業の技術・設備情報の構造化アシスタントです。入力テキストに明示された製品技術・生産設備・生産方式のみをJSON化してください。推測で埋めてはいけません。ITのプログラミング言語を無理に埋めないでください。不明な項目は空配列または空文字にしてください。`,
			fmt.Sprintf(
				`日本の製造企業「%s」の主要技術・製品技術・生産設備・拠点・生産方式を調べてください。公式サイト: %s。会社案内・製品紹介・工場紹介・採用ページに書かれている事実のみを根拠URL付きで列挙してください。プログラミング言語やCI/CDは、公開情報に明示がある場合のみ。`,
				companyName, siteHint,
			),
			"設備・技術"
	}
	return techStackJSONSchemaIT,
		`あなたは技術スタックの構造化アシスタントです。入力テキストに明示された技術のみをJSON化してください。推測で埋めてはいけません。不明な項目は空配列または空文字にしてください。`,
		fmt.Sprintf(
			`日本のIT企業「%s」の技術スタックを調べてください。公式サイト: %s。採用ページ・技術ブログ・エンジニア向け情報・求人票に書かれている言語・フレームワーク・クラウド・CI/CD・開発手法を、根拠URL付きで列挙してください。公開情報として確認できる事実のみ。非上場企業も含めます。`,
			companyName, siteHint,
		),
		"技術スタック"
}

// Acquire は DB 非更新で技術・専門情報を取得する（公式ページ抽出→不足時のみ Search）。
// industry に応じてプロンプトと抽出スキーマを切り替える。
func (f *TechStackFetcher) Acquire(ctx context.Context, companyName, websiteURL, industry string) (*TechStackResult, error) {
	if f.llm == nil || f.llm.Client == nil {
		return nil, fmt.Errorf("openai client is nil")
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	schema, systemPrompt, searchPrompt, domainLabel := techAcquirePrompts(industry, companyName, websiteURL)

	// 1) 公式サイト／採用ページから直接抽出（Search 予算を使わず成功率も高い）
	if strings.TrimSpace(websiteURL) != "" {
		scrapeCtx, scrapeCancel := context.WithTimeout(ctx, 25*time.Second)
		defer scrapeCancel()
		candidateURLs := companyfetch.CandidateCareerURLs(websiteURL)
		if len(candidateURLs) > 3 {
			candidateURLs = candidateURLs[:3]
		}
		if pageText, sourceURL, ferr := companyfetch.FirstFetchableText(scrapeCtx, candidateURLs); ferr == nil && pageText != "" {
			parseUser := fmt.Sprintf(
				"企業「%s」の公開ページ本文です。本文に明示された%sのみ次のJSON形式で抽出してください。推測禁止。\n%s\n\n---\n本文:\n%s",
				companyName, domainLabel, schema, pageText,
			)
			raw, model, err := f.llm.ExtractJSON(ctx, systemPrompt, parseUser, 400)
			if err == nil {
				if result, perr := parseTechStackResult(raw); perr == nil && techResultHasData(result) {
					result.Source = companyfetch.SourceScrape
					result.SourceURL = sourceURL
					result.ModelUsed = model
					result.Confidence = companyfetch.ConfidenceHigh
					return result, nil
				}
			}
		}
	}

	// 2) Search Lite → Parse
	parseUser := fmt.Sprintf(
		"企業「%s」について、検索結果の事実のみに基づき次のJSON形式で回答してください。検索結果に無い項目は空配列/空文字。推測禁止。\n%s",
		companyName, schema,
	)
	raw, modelsUsed, err := f.llm.SearchLiteThenParse(ctx, searchPrompt, systemPrompt, parseUser, 400)
	if err != nil {
		return nil, fmt.Errorf("%sの取得失敗: %w", domainLabel, err)
	}
	result, err := parseTechStackResult(raw)
	if err != nil {
		return nil, err
	}
	result.Source = companyfetch.SourceWebSearch
	result.SourceURL = strings.TrimSpace(websiteURL)
	result.ModelUsed = modelsUsed
	result.Confidence = companyfetch.ConfidenceMedium
	return result, nil
}

func websiteURLOrUnknown(websiteURL string) string {
	if strings.TrimSpace(websiteURL) == "" {
		return "不明"
	}
	return websiteURL
}

func parseTechStackResult(text string) (*TechStackResult, error) {
	obj, err := companyfetch.ExtractJSONObject(text)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ai response: %w", err)
	}
	var result TechStackResult
	if err := json.Unmarshal([]byte(obj), &result); err != nil {
		return nil, fmt.Errorf("failed to parse tech stack json: %w", err)
	}
	// 代替キー（languages / frameworks 等）も吸収
	if !techResultHasData(&result) {
		var alt map[string]any
		if json.Unmarshal([]byte(obj), &alt) == nil {
			result.TechStack = append(result.TechStack, stringSliceFromAny(alt["languages"])...)
			result.TechStack = append(result.TechStack, stringSliceFromAny(alt["frameworks"])...)
			result.TechStack = append(result.TechStack, stringSliceFromAny(alt["technologies"])...)
			if len(result.InfraStack) == 0 {
				result.InfraStack = stringSliceFromAny(alt["infrastructure"])
			}
			if len(result.CicdTools) == 0 {
				result.CicdTools = stringSliceFromAny(alt["ci_cd"])
			}
		}
	}
	result.TechStack = uniqueNonEmpty(result.TechStack)
	result.InfraStack = uniqueNonEmpty(result.InfraStack)
	result.CicdTools = uniqueNonEmpty(result.CicdTools)
	return &result, nil
}

func techResultHasData(r *TechStackResult) bool {
	if r == nil {
		return false
	}
	return len(r.TechStack) > 0 || len(r.InfraStack) > 0 || len(r.CicdTools) > 0 || strings.TrimSpace(r.DevelopmentStyle) != ""
}

func stringSliceFromAny(v any) []string {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				if s = strings.TrimSpace(s); s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	case []string:
		return uniqueNonEmpty(t)
	case string:
		if s := strings.TrimSpace(t); s != "" {
			return []string{s}
		}
	}
	return nil
}

func uniqueNonEmpty(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	return out
}

func techResultFromCompany(company *models.Company) *TechStackResult {
	result := &TechStackResult{
		DevelopmentStyle: company.DevelopmentStyle,
		Source:           company.SourceType,
		SourceURL:        company.SourceURL,
		ModelUsed:        company.LastModelUsed,
		Confidence:       company.LastFetchConfidence,
	}
	_ = json.Unmarshal([]byte(company.TechStack), &result.TechStack)
	_ = json.Unmarshal([]byte(company.InfraStack), &result.InfraStack)
	_ = json.Unmarshal([]byte(company.CicdTools), &result.CicdTools)
	return result
}
