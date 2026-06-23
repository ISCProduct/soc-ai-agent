package services

import (
	"Backend/domain/repository"
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CompanyInfoResult はWebSearchで取得した企業基本情報。
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
}

// CompanyInfoFetcher はOpenAI WebSearchを使って企業基本情報を取得・保存するサービス。
type CompanyInfoFetcher struct {
	repo         repository.CompanyRepository
	openaiClient *openai.Client
}

func NewCompanyInfoFetcher(repo repository.CompanyRepository, client *openai.Client) *CompanyInfoFetcher {
	return &CompanyInfoFetcher{repo: repo, openaiClient: client}
}

// FetchAndSave は指定企業のWebSearch情報を取得してDBに保存し、取得結果を返す。
// 取得失敗時は既存データを変更しない。
func (f *CompanyInfoFetcher) FetchAndSave(ctx context.Context, companyID uint) (*CompanyInfoResult, error) {
	company, err := f.repo.FindByID(companyID)
	if err != nil {
		return nil, fmt.Errorf("company not found: %w", err)
	}

	prompt := fmt.Sprintf(
		`「%s」という企業の情報を調査してください。以下のJSON形式のみで回答してください（余分な説明は不要）。
{
  "description": "企業概要（100〜200文字程度）",
  "industry": "業種（例: IT・ソフトウェア, 金融, 製造業）",
  "location": "本社所在地（例: 東京都渋谷区）",
  "website_url": "公式サイトURL（https://から始まる）",
  "founded_year": 設立年（整数、不明なら0）,
  "employee_count": 従業員数（整数、不明なら0）,
  "main_business": "主要事業内容（50〜100文字程度）",
  "culture": "企業文化・働き方の特徴（50〜100文字程度）",
  "work_style": "勤務スタイル（リモート / ハイブリッド / オフィス のいずれか、不明なら空文字）"
}`,
		company.Name,
	)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	text, err := f.openaiClient.WebSearchQuery(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("web search failed: %w", err)
	}

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("failed to parse web search response: no valid JSON found")
	}

	var result CompanyInfoResult
	if err := json.Unmarshal([]byte(text[start:end+1]), &result); err != nil {
		return nil, fmt.Errorf("failed to parse company info json: %w", err)
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

	if err := f.repo.Update(company); err != nil {
		return nil, fmt.Errorf("failed to update company: %w", err)
	}

	return &result, nil
}
