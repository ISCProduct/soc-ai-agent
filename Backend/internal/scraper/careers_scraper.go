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
	Source          string   `json:"source"` // scrape | web_search
}

// CareersScraper は公式ページのスクレイプ優先、失敗時 Search→Parse で求人を取得する。
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

// FetchJobs はスクレイプ優先、失敗時 Search→Parse で求人一覧を取得する。
func (s *CareersScraper) FetchJobs(ctx context.Context, companyName, websiteURL string) ([]JobPostingResult, error) {
	if s.llm == nil || s.llm.Client == nil {
		return nil, fmt.Errorf("openai client is nil")
	}

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	systemPrompt := `あなたは採用情報の抽出アシスタントです。与えられたテキストに記載のある求人のみをJSON化してください。求人を推測で作り上げてはいけません。求人が無ければ {"jobs": []} を返してください。`

	careerURLs := companyfetch.CandidateCareerURLs(websiteURL)
	// 採用系パスを優先（トップページは後回し）
	prioritized := prioritizeCareerURLs(careerURLs)
	if text, sourceURL, err := companyfetch.FirstFetchableText(prioritized); err == nil {
		userPrompt := fmt.Sprintf(
			"企業「%s」の採用ページテキストから募集職種を抽出し、次のJSON形式のみで回答してください。\n%s\n\n出典URL: %s\n\n---\nテキスト:\n%s",
			companyName, jobsJSONSchema, sourceURL, text,
		)
		raw, _, err := s.llm.ExtractJSON(ctx, systemPrompt, userPrompt, 1500)
		if err == nil {
			jobs, parseErr := parseJobsJSON(raw, companyfetch.SourceScrape, sourceURL)
			if parseErr == nil && len(jobs) > 0 {
				return jobs, nil
			}
		}
	}

	searchPrompt := fmt.Sprintf(
		`日本の企業「%s」の現在公開中の求人・採用職種を調べ、職種名・勤務地・雇用形態・求人URLが分かる範囲で簡潔に列挙してください。公式採用ページのURLがあれば含めてください。公式サイト: %s`,
		companyName, websiteURL,
	)
	parseUser := fmt.Sprintf(
		"企業「%s」について、検索結果に基づき次のJSON形式のみで回答してください。推測の求人は入れないでください。\n%s",
		companyName, jobsJSONSchema,
	)
	raw, _, err := s.llm.SearchThenParse(ctx, searchPrompt, systemPrompt, parseUser, 1500)
	if err != nil {
		return nil, fmt.Errorf("求人情報の取得失敗: %w", err)
	}
	return parseJobsJSON(raw, companyfetch.SourceWebSearch, "")
}

func prioritizeCareerURLs(urls []string) []string {
	if len(urls) <= 1 {
		return urls
	}
	// CandidateCareerURLs は [base, /careers, ...] の順。base を末尾へ。
	out := make([]string, 0, len(urls))
	out = append(out, urls[1:]...)
	out = append(out, urls[0])
	return out
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
