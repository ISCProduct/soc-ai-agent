package scraper

import (
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	Source          string   `json:"source"` // "ai_knowledge"
}

// CareersScraper はAIモデルの知識から企業の求人情報を取得する。
type CareersScraper struct {
	openaiClient *openai.Client
}

// NewCareersScraper は CareersScraper を生成する。
func NewCareersScraper(client *openai.Client) *CareersScraper {
	return &CareersScraper{
		openaiClient: client,
	}
}

// jobsJSONSchema はプロンプトに埋め込む求人JSONスキーマ。
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

// FetchJobs はAIモデルの知識から企業の求人情報をチャット補完1回で取得してJSONに変換する。
func (s *CareersScraper) FetchJobs(ctx context.Context, companyName, websiteURL string) ([]JobPostingResult, error) {
	siteHint := ""
	if websiteURL != "" {
		siteHint = fmt.Sprintf("（公式サイト: %s）", websiteURL)
	}
	systemPrompt := `あなたは日本企業の採用情報に詳しいアシスタントです。確実に知っている情報のみを回答し、不明な値は空文字・0・空配列にしてください。求人を推測で作り上げてはいけません。`
	userPrompt := fmt.Sprintf(
		`「%s」%sが募集していることで知られる職種の一覧を、以下のJSON形式のみで回答してください（説明文は不要）。求人情報を知らない場合は {"jobs": []} を返してください。%s`,
		companyName, siteHint, jobsJSONSchema,
	)

	result, err := s.openaiClient.ChatCompletionJSON(ctx, systemPrompt, userPrompt, 0.2, 1500)
	if err != nil {
		return nil, fmt.Errorf("求人情報の取得失敗: %w", err)
	}
	if strings.TrimSpace(result) == "" {
		return nil, nil
	}

	// レスポンスからJSONオブジェクトを抽出
	start := strings.Index(result, "{")
	end := strings.LastIndex(result, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, nil
	}

	var parsed struct {
		Jobs []JobPostingResult `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(result[start:end+1]), &parsed); err != nil {
		return nil, fmt.Errorf("求人JSONの解析失敗: %w", err)
	}

	for i := range parsed.Jobs {
		parsed.Jobs[i].Source = "ai_knowledge"
	}
	return parsed.Jobs, nil
}
