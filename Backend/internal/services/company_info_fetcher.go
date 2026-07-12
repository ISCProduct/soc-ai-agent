package services

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"Backend/internal/models"
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CompanyInfoResult は企業基本情報の取得結果。
type CompanyInfoResult struct {
	Description   string `json:"description"`
	Industry      string `json:"industry"`
	Location      string `json:"location"`
	WebsiteURL    string `json:"website_url"`
	FoundedYear   int    `json:"founded_year"`
	EmployeeCount int    `json:"employee_count"`
	MainBusiness  string `json:"main_business"`
	Culture       string `json:"culture"`
	WorkStyle     string `json:"work_style"`
	Source        string `json:"source,omitempty"`
	SourceURL     string `json:"source_url,omitempty"`
	ModelUsed     string `json:"model_used,omitempty"`
	Confidence    string `json:"confidence,omitempty"`
}

// CompanyInfoFetcher は gBizINFO 優先で企業基本情報を取得・保存する。
// OpenAI Web Search は使わない（コスト回避）。不足フィールドのみ安価な Extract モデルで補完する。
type CompanyInfoFetcher struct {
	repo repository.CompanyRepository
	llm  *companyfetch.LLM
	gbiz *GBizInfoService
}

func NewCompanyInfoFetcher(repo repository.CompanyRepository, client *openai.Client, gbiz ...*GBizInfoService) *CompanyInfoFetcher {
	f := &CompanyInfoFetcher{repo: repo, llm: &companyfetch.LLM{Client: client}}
	if len(gbiz) > 0 {
		f.gbiz = gbiz[0]
	}
	return f
}

const companyInfoJSONSchema = `{
  "description": "企業概要（100〜200文字程度）",
  "industry": "業種（例: IT・ソフトウェア, 金融, 製造業）",
  "location": "本社所在地（例: 東京都渋谷区）",
  "website_url": "公式サイトURL（https://から始まる）",
  "founded_year": 設立年（整数、不明なら0）,
  "employee_count": 従業員数（整数、不明なら0）,
  "main_business": "主要事業内容（50〜100文字程度）",
  "culture": "企業文化・働き方の特徴（50〜100文字程度）",
  "work_style": "勤務スタイル（リモート / ハイブリッド / オフィス のいずれか、不明なら空文字）"
}`

// FetchAndSave は指定企業の基本情報を返す。
// forceRefresh=false かつ InfoFetchedAt が TTL 内なら AI を呼ばずに既存データを返す。
func (f *CompanyInfoFetcher) FetchAndSave(ctx context.Context, companyID uint, forceRefresh bool) (*CompanyInfoResult, error) {
	company, err := f.repo.FindByID(companyID)
	if err != nil {
		return nil, fmt.Errorf("company not found: %w", err)
	}

	if !forceRefresh && companyfetch.IsFresh(company.InfoFetchedAt, companyfetch.TTLInfo) {
		return companyInfoFromModel(company), nil
	}

	result, err := f.acquireForCompany(ctx, company)
	if err != nil {
		return nil, err
	}

	applyCompanyInfoResult(company, result)
	now := time.Now()
	company.SourceType = result.Source
	if result.SourceURL != "" {
		company.SourceURL = result.SourceURL
	}
	company.SourceFetchedAt = &now
	company.InfoFetchedAt = &now
	company.LastModelUsed = result.ModelUsed
	company.LastFetchConfidence = result.Confidence

	if err := f.repo.Update(company); err != nil {
		return nil, fmt.Errorf("failed to update company: %w", err)
	}
	return result, nil
}

// Acquire は企業名から基本情報をプレビュー取得する（DB 非更新）。
func (f *CompanyInfoFetcher) Acquire(ctx context.Context, companyName, websiteURL string) (*CompanyInfoResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	if f.gbiz != nil {
		if result, err := f.acquireFromGBizName(ctx, companyName); err == nil {
			return result, nil
		}
	}

	return f.acquireCheapExtract(ctx, companyName, websiteURL, "")
}

func (f *CompanyInfoFetcher) acquireForCompany(ctx context.Context, company *models.Company) (*CompanyInfoResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	if f.gbiz != nil {
		if result, err := f.acquireFromGBizCompany(ctx, company); err == nil {
			return result, nil
		}
	}

	facts := buildCompanyFactsText(company)
	// 名前以外の根拠が無い場合は空扱い（confidence=low）
	if company.CorporateNumber == "" && company.Location == "" && company.Description == "" &&
		company.WebsiteURL == "" && company.EmployeeCount == 0 && company.FoundedYear == 0 {
		facts = ""
	}
	return f.acquireCheapExtract(ctx, company.Name, company.WebsiteURL, facts)
}

func (f *CompanyInfoFetcher) acquireFromGBizCompany(ctx context.Context, company *models.Company) (*CompanyInfoResult, error) {
	if strings.TrimSpace(company.CorporateNumber) == "" {
		hits, err := f.gbiz.SearchByName(ctx, company.Name)
		if err != nil || len(hits) == 0 {
			return nil, fmt.Errorf("gbizinfo: corporate number not found for %s", company.Name)
		}
		company.CorporateNumber = hits[0].CorporateNumber
		if company.WebsiteURL == "" && hits[0].CompanyURL != "" {
			company.WebsiteURL = hits[0].CompanyURL
		}
		if company.Location == "" && hits[0].Location != "" {
			company.Location = hits[0].Location
		}
		if company.EmployeeCount == 0 && hits[0].EmployeeNumber > 0 {
			company.EmployeeCount = hits[0].EmployeeNumber
		}
		_ = f.repo.Update(company)
	}

	if _, err := f.gbiz.SyncCompany(ctx, company.ID); err != nil {
		return nil, err
	}
	updated, err := f.repo.FindByID(company.ID)
	if err != nil {
		return nil, err
	}
	*company = *updated

	result := companyInfoFromModel(company)
	result.Source = companyfetch.SourceGBiz
	result.Confidence = companyfetch.ConfidenceHigh
	result.ModelUsed = "gbizinfo"

	// gBiz に無い叙述フィールドのみ、取得済み事実テキストから安価 Extract
	if result.Description == "" || result.MainBusiness == "" {
		facts := buildCompanyFactsText(company)
		if enriched, err := f.acquireCheapExtract(ctx, company.Name, company.WebsiteURL, facts); err == nil {
			if result.Description == "" {
				result.Description = enriched.Description
			}
			if result.MainBusiness == "" {
				result.MainBusiness = enriched.MainBusiness
			}
			if result.Industry == "" {
				result.Industry = enriched.Industry
			}
			if result.Culture == "" {
				result.Culture = enriched.Culture
			}
			if result.WorkStyle == "" {
				result.WorkStyle = enriched.WorkStyle
			}
			if enriched.ModelUsed != "" {
				result.ModelUsed = "gbizinfo+" + enriched.ModelUsed
			}
		}
	}
	return result, nil
}

func (f *CompanyInfoFetcher) acquireFromGBizName(ctx context.Context, companyName string) (*CompanyInfoResult, error) {
	hits, err := f.gbiz.SearchByName(ctx, companyName)
	if err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("gbizinfo: no match for %s", companyName)
	}
	hit := hits[0]
	result := &CompanyInfoResult{
		Location:      hit.Location,
		WebsiteURL:    hit.CompanyURL,
		EmployeeCount: hit.EmployeeNumber,
		Source:        companyfetch.SourceGBiz,
		Confidence:    companyfetch.ConfidenceHigh,
		ModelUsed:     "gbizinfo",
	}
	facts := fmt.Sprintf("正式名称: %s\n所在地: %s\n公式URL: %s\n従業員数: %d\n法人番号: %s",
		hit.Name, hit.Location, hit.CompanyURL, hit.EmployeeNumber, hit.CorporateNumber)
	if enriched, err := f.acquireCheapExtract(ctx, companyName, hit.CompanyURL, facts); err == nil {
		result.Description = enriched.Description
		result.Industry = enriched.Industry
		result.MainBusiness = enriched.MainBusiness
		result.Culture = enriched.Culture
		result.WorkStyle = enriched.WorkStyle
		result.FoundedYear = enriched.FoundedYear
		if enriched.ModelUsed != "" {
			result.ModelUsed = "gbizinfo+" + enriched.ModelUsed
		}
	}
	return result, nil
}

// acquireCheapExtract は OpenAI Web Search を使わず Extract モデルのみで JSON 化する。
// factsText がある場合はそれに根拠を限定する。無い場合は不明を空にし推測禁止（confidence=low）。
func (f *CompanyInfoFetcher) acquireCheapExtract(ctx context.Context, companyName, websiteURL, factsText string) (*CompanyInfoResult, error) {
	if f.llm == nil || f.llm.Client == nil {
		return nil, fmt.Errorf("openai client is nil（gBizINFO も未設定のため取得不可）")
	}

	systemPrompt := `あなたは企業情報の構造化アシスタントです。与えられた事実テキストに書かれている内容のみをJSON化してください。テキストに無い項目や不確かな項目は空文字または0にしてください。Web検索や推測で埋めてはいけません。`
	userPrompt := fmt.Sprintf(
		"企業名「%s」（参考URL: %s）について、次の事実テキストのみを根拠にJSON化してください。\n%s\n\n事実テキスト:\n%s",
		companyName, websiteURL, companyInfoJSONSchema, strings.TrimSpace(factsText),
	)
	if strings.TrimSpace(factsText) == "" {
		systemPrompt = `あなたは企業情報の構造化アシスタントです。確実な公開事実のみをJSON化してください。不確かな項目は空文字または0。推測禁止。Web検索ツールは使えません。`
		userPrompt = fmt.Sprintf(
			"企業名「%s」（参考URL: %s）について、確実に分かる項目のみ次のJSON形式で回答してください。不明は空/0。\n%s",
			companyName, websiteURL, companyInfoJSONSchema,
		)
	}

	raw, model, err := f.llm.ExtractJSON(ctx, systemPrompt, userPrompt, 600)
	if err != nil {
		return nil, fmt.Errorf("企業情報の取得失敗: %w", err)
	}
	result, err := parseCompanyInfoResult(raw)
	if err != nil {
		return nil, err
	}
	result.Source = companyfetch.SourceLLMExtract
	result.ModelUsed = model
	if strings.TrimSpace(factsText) != "" {
		result.Confidence = companyfetch.ConfidenceMedium
	} else {
		result.Confidence = companyfetch.ConfidenceLow
	}
	return result, nil
}

func parseCompanyInfoResult(text string) (*CompanyInfoResult, error) {
	obj, err := companyfetch.ExtractJSONObject(text)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ai response: %w", err)
	}
	var result CompanyInfoResult
	if err := json.Unmarshal([]byte(obj), &result); err != nil {
		return nil, fmt.Errorf("failed to parse company info json: %w", err)
	}
	return &result, nil
}

func companyInfoFromModel(company *models.Company) *CompanyInfoResult {
	return &CompanyInfoResult{
		Description:   company.Description,
		Industry:      company.Industry,
		Location:      company.Location,
		WebsiteURL:    company.WebsiteURL,
		FoundedYear:   company.FoundedYear,
		EmployeeCount: company.EmployeeCount,
		MainBusiness:  company.MainBusiness,
		Culture:       company.Culture,
		WorkStyle:     company.WorkStyle,
		Source:        company.SourceType,
		SourceURL:     company.SourceURL,
		ModelUsed:     company.LastModelUsed,
		Confidence:    company.LastFetchConfidence,
	}
}

func applyCompanyInfoResult(company *models.Company, result *CompanyInfoResult) {
	if result.Description != "" {
		company.Description = result.Description
	}
	if result.Industry != "" {
		company.Industry = result.Industry
	}
	if result.Location != "" {
		company.Location = result.Location
	}
	if result.WebsiteURL != "" {
		company.WebsiteURL = result.WebsiteURL
	}
	if result.FoundedYear > 0 {
		company.FoundedYear = result.FoundedYear
	}
	if result.EmployeeCount > 0 {
		company.EmployeeCount = result.EmployeeCount
	}
	if result.MainBusiness != "" {
		company.MainBusiness = result.MainBusiness
	}
	if result.Culture != "" {
		company.Culture = result.Culture
	}
	if result.WorkStyle != "" {
		company.WorkStyle = result.WorkStyle
	}
}

func buildCompanyFactsText(company *models.Company) string {
	var b strings.Builder
	fmt.Fprintf(&b, "正式名称: %s\n", company.Name)
	if company.CorporateNumber != "" {
		fmt.Fprintf(&b, "法人番号: %s\n", company.CorporateNumber)
	}
	if company.Location != "" {
		fmt.Fprintf(&b, "所在地: %s\n", company.Location)
	}
	if company.WebsiteURL != "" {
		fmt.Fprintf(&b, "公式URL: %s\n", company.WebsiteURL)
	}
	if company.FoundedYear > 0 {
		fmt.Fprintf(&b, "設立年: %d\n", company.FoundedYear)
	}
	if company.EmployeeCount > 0 {
		fmt.Fprintf(&b, "従業員数: %d\n", company.EmployeeCount)
	}
	if company.Industry != "" {
		fmt.Fprintf(&b, "業種: %s\n", company.Industry)
	}
	if company.Description != "" {
		fmt.Fprintf(&b, "概要: %s\n", company.Description)
	}
	if company.MainBusiness != "" {
		fmt.Fprintf(&b, "主要事業: %s\n", company.MainBusiness)
	}
	return b.String()
}
