package scraper

import (
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/time/rate"
)

// JobPostingResult は1件の求人情報を表す。
type JobPostingResult struct {
	Title           string   `json:"title"`
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

// FetchFromCareersPage は企業の公式採用ページから求人情報を取得する。
func (s *CareersScraper) FetchFromCareersPage(ctx context.Context, companyName, websiteURL string) ([]JobPostingResult, error) {
	careersURL, err := s.findCareersPageURL(ctx, companyName, websiteURL)
	if err != nil {
		return nil, fmt.Errorf("採用ページURL取得失敗: %w", err)
	}
	if careersURL == "" {
		return nil, nil
	}

	body, err := FetchHTML(careersURL, "SOC-AI-Agent/1.0 (recruitment-research)", s.limiter)
	if err != nil {
		return nil, fmt.Errorf("採用ページ取得失敗 %s: %w", careersURL, err)
	}

	text := extractTextFromHTML(body)
	// LLMへの入力量を制限
	if runes := []rune(text); len(runes) > 8000 {
		text = string(runes[:8000])
	}

	return s.parseJobsFromText(ctx, companyName, text, "careers_page")
}

// FetchFromWantedly はWantedlyのWebSearch結果から求人情報を取得する。
func (s *CareersScraper) FetchFromWantedly(ctx context.Context, companyName string) ([]JobPostingResult, error) {
	query := fmt.Sprintf(`site:wantedly.com "%s" 募集職種`, companyName)
	result, err := s.openaiClient.WebSearchQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("wantedly検索失敗: %w", err)
	}
	if strings.TrimSpace(result) == "" {
		return nil, nil
	}

	return s.parseJobsFromText(ctx, companyName, result, "wantedly")
}

func (s *CareersScraper) findCareersPageURL(ctx context.Context, companyName, websiteURL string) (string, error) {
	var query string
	if websiteURL != "" {
		query = fmt.Sprintf(`「%s」（%s）の採用・求人ページのURLを1つ教えてください。URLのみ回答してください。`, companyName, websiteURL)
	} else {
		query = fmt.Sprintf(`「%s」 採用 求人 site:company recruit OR careers OR jobs。採用ページのURLを1つだけ回答してください。`, companyName)
	}

	result, err := s.openaiClient.WebSearchQuery(ctx, query)
	if err != nil {
		return "", err
	}

	for line := range strings.SplitSeq(result, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https://") || strings.HasPrefix(line, "http://") {
			return line, nil
		}
	}
	return "", nil
}

func (s *CareersScraper) parseJobsFromText(ctx context.Context, companyName, text, source string) ([]JobPostingResult, error) {
	systemPrompt := `あなたは求人情報解析の専門家です。与えられたテキストから求人情報を抽出し、指定のJSON形式のみで回答してください。`
	userPrompt := fmt.Sprintf(`以下は「%s」の採用情報です。求人情報を抽出して以下のJSON配列形式のみで回答してください（説明文は不要）。

[
  {
    "title": "職種名",
    "employment_type": "正社員 or 契約社員 or アルバイト",
    "work_location": "勤務地（例: 東京都渋谷区）",
    "remote_option": true or false,
    "min_salary": 最低年収（万円・整数、不明なら0）,
    "max_salary": 最高年収（万円・整数、不明なら0）,
    "required_skills": ["必須スキル1", "必須スキル2"],
    "preferred_skills": ["歓迎スキル1", "歓迎スキル2"],
    "description": "職種説明（100文字程度）",
    "persona_keywords": ["求める人物像キーワード1", "求める人物像キーワード2"]
  }
]

採用情報テキスト:
%s`, companyName, text)

	jsonStr, err := s.openaiClient.ChatCompletionJSON(ctx, systemPrompt, userPrompt, 0.3, 2000)
	if err != nil {
		return nil, fmt.Errorf("求人情報解析失敗: %w", err)
	}

	start := strings.Index(jsonStr, "[")
	end := strings.LastIndex(jsonStr, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("JSONレスポンスのパース失敗: %q", jsonStr[:min(len(jsonStr), 200)])
	}

	var results []JobPostingResult
	if err := json.Unmarshal([]byte(jsonStr[start:end+1]), &results); err != nil {
		return nil, fmt.Errorf("JSON unmarshal失敗: %w", err)
	}

	for i := range results {
		results[i].Source = source
	}
	return results, nil
}

// extractTextFromHTML はHTMLのテキストノードを結合して返す。script/styleは除外する。
func extractTextFromHTML(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return htmlStr
	}
	var buf strings.Builder
	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		if n.Type == html.TextNode {
			if text := strings.TrimSpace(n.Data); text != "" {
				buf.WriteString(text)
				buf.WriteByte(' ')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(doc)
	return buf.String()
}

