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
}

// TechStackFetcher は安価な Extract モデルのみで技術スタックを取得する（OpenAI Web Search 不使用）。
type TechStackFetcher struct {
	repo repository.CompanyRepository
	llm  *companyfetch.LLM
}

func NewTechStackFetcher(repo repository.CompanyRepository, client *openai.Client) *TechStackFetcher {
	return &TechStackFetcher{repo: repo, llm: &companyfetch.LLM{Client: client}}
}

const techStackJSONSchema = `{
  "tech_stack": ["言語・フレームワーク名（例: Go, React, TypeScript）"],
  "infra_stack": ["インフラ名（例: AWS, GCP, Azure, オンプレ）"],
  "cicd_tools": ["CI/CDツール名（例: GitHub Actions, Jenkins, CircleCI）"],
  "development_style": "開発手法（例: スクラム, ウォーターフォール, カンバン）"
}`

// FetchAndSave は技術スタックを取得して DB を更新する。
func (f *TechStackFetcher) FetchAndSave(ctx context.Context, companyID uint, forceRefresh bool) (*TechStackResult, error) {
	company, err := f.repo.FindByID(companyID)
	if err != nil {
		return nil, fmt.Errorf("company not found: %w", err)
	}

	if !forceRefresh && companyfetch.IsFresh(company.TechFetchedAt, companyfetch.TTLTech) {
		return techResultFromCompany(company), nil
	}

	result, err := f.Acquire(ctx, company.Name, company.WebsiteURL)
	if err != nil {
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
	company.TechFetchedAt = &now
	company.SourceFetchedAt = &now
	if result.Source != "" {
		company.SourceType = result.Source
	}
	company.LastModelUsed = result.ModelUsed
	company.LastFetchConfidence = result.Confidence

	if err := f.repo.Update(company); err != nil {
		return nil, fmt.Errorf("failed to update company: %w", err)
	}
	return result, nil
}

// Acquire は DB 非更新で技術スタックを取得する（Web Search なし）。
func (f *TechStackFetcher) Acquire(ctx context.Context, companyName, websiteURL string) (*TechStackResult, error) {
	if f.llm == nil || f.llm.Client == nil {
		return nil, fmt.Errorf("openai client is nil")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	systemPrompt := `あなたは技術スタックの構造化アシスタントです。確実に公開されていると分かる技術のみをJSON化してください。不確かな項目は空配列または空文字。Web検索ツールは使えません。推測禁止。`
	siteHint := websiteURL
	if siteHint == "" {
		siteHint = "不明"
	}
	userPrompt := fmt.Sprintf(
		"企業「%s」（公式サイト: %s）について、確実な技術情報のみ次のJSON形式で回答してください。\n%s",
		companyName, siteHint, techStackJSONSchema,
	)
	raw, model, err := f.llm.ExtractJSON(ctx, systemPrompt, userPrompt, 400)
	if err != nil {
		return nil, fmt.Errorf("技術スタックの取得失敗: %w", err)
	}
	result, err := parseTechStackResult(raw)
	if err != nil {
		return nil, err
	}
	result.Source = companyfetch.SourceLLMExtract
	result.ModelUsed = model
	result.Confidence = companyfetch.ConfidenceLow
	return result, nil
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
	return &result, nil
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
