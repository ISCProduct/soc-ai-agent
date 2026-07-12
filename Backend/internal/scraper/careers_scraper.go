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
	Source          string   `json:"source"` // web_search (mini-search)
}

// CareersScraper は安価な AI Search（mini-search→Parse）で求人を取得する。
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

// FetchJobs は mini-search → Parse で公開求人の事実のみを取得する（deep search なし）。
func (s *CareersScraper) FetchJobs(ctx context.Context, companyName, websiteURL string) ([]JobPostingResult, error) {
	if s.llm == nil || s.llm.Client == nil {
		return nil, fmt.Errorf("openai client is nil")
	}

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	systemPrompt := `あなたは採用情報の構造化アシスタントです。検索結果に明示された公開求人のみをJSON化してください。求人を推測で作ってはいけません。無ければ {"jobs": []} を返してください。`
	siteHint := websiteURL
	if siteHint == "" {
		siteHint = "不明"
	}
	searchPrompt := fmt.Sprintf(
		`日本の企業「%s」の現在公開中の求人・採用職種を調べてください。公式サイト: %s。職種名・勤務地・雇用形態・求人URLが公開情報として確認できるものだけを列挙し、根拠URLを含めてください。非上場企業も含め、確認できない求人は含めないでください。`,
		companyName, siteHint,
	)
	parseUser := fmt.Sprintf(
		"企業「%s」について、検索結果の事実のみに基づき次のJSON形式で回答してください。推測の求人は入れないでください。\n%s",
		companyName, jobsJSONSchema,
	)
	raw, _, err := s.llm.SearchLiteThenParse(ctx, searchPrompt, systemPrompt, parseUser, 1500)
	if err != nil {
		return nil, fmt.Errorf("求人情報の取得失敗: %w", err)
	}
	return parseJobsJSON(raw, companyfetch.SourceWebSearch, "")
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
