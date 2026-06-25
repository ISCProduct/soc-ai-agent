package scraper

import (
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/time/rate"
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
	Source          string   `json:"source"` // "careers_page" or "wantedly"
}

// CareersScraper は企業の採用ページとWantedlyから求人情報を取得する。
type CareersScraper struct {
	openaiClient *openai.Client
	limiter      *rate.Limiter
}

// NewCareersScraper は CareersScraper を生成する。
func NewCareersScraper(client *openai.Client) *CareersScraper {
	return &CareersScraper{
		openaiClient: client,
		limiter:      NewLimiter(2 * time.Second),
	}
}

// jobsJSONSchema はWebSearchプロンプトに埋め込む求人JSONスキーマ。
const jobsJSONSchema = `
[
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
]`

// FetchFromCareersPage は企業の採用情報をWebSearch 1回で取得してJSONに変換する。
func (s *CareersScraper) FetchFromCareersPage(ctx context.Context, companyName, websiteURL string) ([]JobPostingResult, error) {
	siteHint := ""
	if websiteURL != "" {
		siteHint = fmt.Sprintf("（公式サイト: %s）", websiteURL)
	}
	query := fmt.Sprintf(
		`「%s」%sの採用・求人情報を調べて、募集職種の一覧を以下のJSON配列形式のみで回答してください（説明文は不要）。%s`,
		companyName, siteHint, jobsJSONSchema,
	)

	return s.fetchAndParseJobs(ctx, query, "careers_page")
}

// FetchFromWantedly はWantedlyから企業の求人情報をWebSearch 1回で取得してJSONに変換する。
func (s *CareersScraper) FetchFromWantedly(ctx context.Context, companyName string) ([]JobPostingResult, error) {
	query := fmt.Sprintf(
		`site:wantedly.com の「%s」の募集職種一覧を以下のJSON配列形式のみで回答してください（説明文は不要）。%s`,
		companyName, jobsJSONSchema,
	)

	return s.fetchAndParseJobs(ctx, query, "wantedly")
}

// fetchAndParseJobs は WebSearch クエリを実行してレスポンスから JSON 配列を抽出する。
func (s *CareersScraper) fetchAndParseJobs(ctx context.Context, query, source string) ([]JobPostingResult, error) {
	result, err := s.openaiClient.WebSearchQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("WebSearch失敗(%s): %w", source, err)
	}
	if strings.TrimSpace(result) == "" {
		return nil, nil
	}

	// WebSearchレスポンスからJSON配列を抽出
	start := strings.Index(result, "[")
	end := strings.LastIndex(result, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, nil
	}

	var jobs []JobPostingResult
	if err := json.Unmarshal([]byte(result[start:end+1]), &jobs); err != nil {
		return nil, fmt.Errorf("JSON解析失敗(%s): %w", source, err)
	}

	for i := range jobs {
		jobs[i].Source = source
	}
	return jobs, nil
}
