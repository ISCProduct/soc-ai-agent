package services

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"Backend/internal/models"
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"fmt"
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

// CompanyInfoFetcher はスクレイプ/Search テキストから企業基本情報を抽出・保存する。
type CompanyInfoFetcher struct {
	repo repository.CompanyRepository
	llm  *companyfetch.LLM
}

func NewCompanyInfoFetcher(repo repository.CompanyRepository, client *openai.Client) *CompanyInfoFetcher {
	return &CompanyInfoFetcher{repo: repo, llm: &companyfetch.LLM{Client: client}}
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

	result, err := f.Acquire(ctx, company.Name, company.WebsiteURL)
	if err != nil {
		return nil, err
	}

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

// Acquire は企業名から Web Search→Parse で基本情報を取得する（DB 非更新）。
// Phase 1 暫定: スクレイプは運用コストが高いため使わず、OpenAI Search で事実のみを収集する。
func (f *CompanyInfoFetcher) Acquire(ctx context.Context, companyName, websiteURL string) (*CompanyInfoResult, error) {
	if f.llm == nil || f.llm.Client == nil {
		return nil, fmt.Errorf("openai client is nil")
	}

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	systemPrompt := `あなたは企業情報の構造化アシスタントです。検索結果に明示された事実のみをJSON化してください。検索結果に無い項目は空文字または0にしてください。モデルの事前知識や推測で埋めてはいけません。`

	siteHint := ""
	if websiteURL != "" {
		siteHint = fmt.Sprintf("（参考公式URL: %s）", websiteURL)
	}
	searchPrompt := fmt.Sprintf(
		`日本の企業「%s」%sについて、公開情報から確認できる事実だけを調べてください。対象: 企業概要・業種・本社所在地・公式サイトURL・設立年・従業員数・主要事業・企業文化・勤務スタイル。各事実の根拠URLを必ず含めてください。不明な項目は推測せず「不明」と書いてください。`,
		companyName, siteHint,
	)
	parseUser := fmt.Sprintf(
		"企業名「%s」について、検索結果の事実のみに基づき次のJSON形式で回答してください。検索結果に無い項目は空文字または0。推測禁止。\n%s",
		companyName, companyInfoJSONSchema,
	)
	raw, modelsUsed, err := f.llm.SearchThenParse(ctx, searchPrompt, systemPrompt, parseUser, 600)
	if err != nil {
		return nil, fmt.Errorf("企業情報の取得失敗: %w", err)
	}
	result, err := parseCompanyInfoResult(raw)
	if err != nil {
		return nil, err
	}
	result.Source = companyfetch.SourceWebSearch
	result.ModelUsed = modelsUsed
	result.Confidence = companyfetch.ConfidenceMedium
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
