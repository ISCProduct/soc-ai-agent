package scraper

import (
	"Backend/internal/companyfetch"
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// JobPostingResult は1件の求人情報を表す。
type JobPostingResult struct {
	Title           string   `json:"title"`
	URL             string   `json:"url"`
	EmploymentType  string   `json:"employment_type"`
	WorkLocation    string   `json:"work_location"`
	RemoteOption    bool     `json:"remote_option"`
	MinSalary       int      `json:"min_salary"`
	MaxSalary       int      `json:"max_salary"`
	RequiredSkills  []string `json:"required_skills"`
	PreferredSkills []string `json:"preferred_skills"`
	Description     string   `json:"description"`
	PersonaKeywords []string `json:"persona_keywords"`
	Source          string   `json:"source"` // llm_extract（WebSearchなし）
}

// CareersScraper は安価な Extract モデルのみで求人候補を返す（OpenAI Web Search 不使用）。
// 鮮度保証はない。確かな公開求人のみ・不明は空。
type CareersScraper struct {
	llm *companyfetch.LLM
}

// NewCareersScraper は CareersScraper を生成する。
func NewCareersScraper(client *openai.Client) *CareersScraper {
	return &CareersScraper{llm: &companyfetch.LLM{Client: client}}
}

const jobsJSONSchema = `
{
  "jobs": [
    {
      "title": "職種名（必須・空文字不可）",
      "url": "求人ページのURL（不明なら空文字）",
      "employment_type": "正社員 or 契約社員 or アルバイト",
      "work_location": "勤務地",
      "remote_option": true or false,
      "min_salary": 最低年収（万円・整数、不明なら0）,
      "max_salary": 最高年収（万円・整数、不明なら0）,
      "required_skills": ["必須スキル"],
      "preferred_skills": ["歓迎スキル"],
      "description": "職種説明（100文字程度）",
      "persona_keywords": ["求める人物像キーワード"]
    }
  ]
}`

// FetchJobs は OpenAI Web Search を使わず Extract モデルのみで求人を返す。
func (s *CareersScraper) FetchJobs(ctx context.Context, companyName, websiteURL string) ([]JobPostingResult, error) {
	if s.llm == nil || s.llm.Client == nil {
		return nil, fmt.Errorf("openai client is nil")
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	systemPrompt := `あなたは採用情報の構造化アシスタントです。確実に公開されていると分かる求人のみをJSON化してください。不確かな求人は含めず、無ければ {"jobs": []} を返してください。Web検索ツールは使えません。推測で求人を作ってはいけません。`
	siteHint := websiteURL
	if siteHint == "" {
		siteHint = "不明"
	}
	userPrompt := fmt.Sprintf(
		"企業「%s」（公式サイト: %s）について、確実な公開求人のみ次のJSON形式で回答してください。\n%s",
		companyName, siteHint, jobsJSONSchema,
	)
	raw, _, err := s.llm.ExtractJSON(ctx, systemPrompt, userPrompt, 1500)
	if err != nil {
		return nil, fmt.Errorf("求人情報の取得失敗: %w", err)
	}
	return parseJobsJSON(raw, companyfetch.SourceLLMExtract, "")
}

func parseJobsJSON(text, source, defaultURL string) ([]JobPostingResult, error) {
	obj, err := companyfetch.ExtractJSONObject(text)
	if err != nil {
		return nil, nil
	}
	var parsed struct {
		Jobs []JobPostingResult `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(obj), &parsed); err != nil {
		return nil, fmt.Errorf("求人JSONの解析失敗: %w", err)
	}
	for i := range parsed.Jobs {
		parsed.Jobs[i].Source = source
		if strings.TrimSpace(parsed.Jobs[i].URL) == "" && defaultURL != "" {
			parsed.Jobs[i].URL = defaultURL
		}
	}
	return parsed.Jobs, nil
}
