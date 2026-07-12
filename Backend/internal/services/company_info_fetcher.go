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

// Acquire は企業名と任意の公式 URL から基本情報を取得する（DB 非更新）。
func (f *CompanyInfoFetcher) Acquire(ctx context.Context, companyName, websiteURL string) (*CompanyInfoResult, error) {
	if f.llm == nil || f.llm.Client == nil {
		return nil, fmt.Errorf("openai client is nil")
	}

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	systemPrompt := `あなたは企業情報の抽出アシスタントです。与えられたテキストに書かれている事実のみをJSONで抽出してください。テキストに無い項目は空文字または0にしてください。推測で埋めてはいけません。`

	if text, sourceURL, err := companyfetch.FirstFetchableText(companyfetch.CandidateCareerURLs(websiteURL)); err == nil {
		userPrompt := fmt.Sprintf(
			"企業名「%s」について、以下の公式サイトテキストから情報を抽出し、次のJSON形式のみで回答してください。\n%s\n\n---\nテキスト:\n%s",
			companyName, companyInfoJSONSchema, text,
		)
		raw, model, err := f.llm.ExtractJSON(ctx, systemPrompt, userPrompt, 600)
		if err == nil {
			if result, parseErr := parseCompanyInfoResult(raw); parseErr == nil && result.Description != "" {
				result.Source = companyfetch.SourceScrape
				result.SourceURL = sourceURL
				result.ModelUsed = model
				result.Confidence = companyfetch.ConfidenceHigh
				return result, nil
			}
		}
	}

	searchPrompt := fmt.Sprintf(
		`日本の企業「%s」の最新の企業概要・業種・本社所在地・公式サイトURL・設立年・従業員数・主要事業・企業文化・勤務スタイルを調べ、根拠となるURLを含めて簡潔にまとめてください。`,
		companyName,
	)
	parseUser := fmt.Sprintf(
		"企業名「%s」について、検索結果に基づき次のJSON形式のみで回答してください。不明な項目は空文字または0。\n%s",
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
