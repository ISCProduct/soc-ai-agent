package companyfetch

import (
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// BraveSearchProvider は Brave Search API でスニペットを取得し、必要なら Parse 用テキストを返す。
// OpenAI Search ツール料金を避けるための差し込み実装（#590）。
type BraveSearchProvider struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	// ParseClient は任意。設定時は Brave 結果を JSON っぽく整えるために mini Parse を使えるが、
	// 既定では生スニペット連結のみ（追加 LLM 課金なし）。
	ParseClient *openai.Client
}

func NewBraveSearchProvider(apiKey string, parseClient *openai.Client) *BraveSearchProvider {
	base := strings.TrimSpace(os.Getenv("BRAVE_SEARCH_BASE_URL"))
	if base == "" {
		base = "https://api.search.brave.com/res/v1/web/search"
	}
	return &BraveSearchProvider{
		APIKey:      apiKey,
		BaseURL:     base,
		HTTPClient:  &http.Client{Timeout: 20 * time.Second},
		ParseClient: parseClient,
	}
}

func (p *BraveSearchProvider) Name() string { return "brave" }

type braveSearchResponse struct {
	Web *struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"results"`
	} `json:"web"`
}

func (p *BraveSearchProvider) Search(ctx context.Context, query string, maxTokens int) (string, string, error) {
	if p == nil || strings.TrimSpace(p.APIKey) == "" {
		return "", "", fmt.Errorf("brave search provider: api key is empty")
	}
	if maxTokens <= 0 {
		maxTokens = 1200
	}
	endpoint, err := url.Parse(p.BaseURL)
	if err != nil {
		return "", "", providerSearchError("brave", err)
	}
	q := endpoint.Query()
	q.Set("q", query)
	q.Set("count", "8")
	q.Set("search_lang", "jp")
	q.Set("country", "JP")
	endpoint.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", "", providerSearchError("brave", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", p.APIKey)

	client := p.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", providerSearchError("brave", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", providerSearchError("brave", err)
	}
	if resp.StatusCode >= 300 {
		return "", "", providerSearchError("brave", fmt.Errorf("status=%d body=%s", resp.StatusCode, TrimText(string(body), 200)))
	}

	var parsed braveSearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", providerSearchError("brave", err)
	}
	var b strings.Builder
	b.WriteString("Brave Search 結果:\n")
	count := 0
	if parsed.Web != nil {
		for _, r := range parsed.Web.Results {
			count++
			fmt.Fprintf(&b, "%d. %s\nURL: %s\n%s\n\n", count, r.Title, r.URL, r.Description)
		}
	}
	text := strings.TrimSpace(b.String())
	if text == "" || count == 0 {
		return "", "", providerSearchError("brave", fmt.Errorf("no results"))
	}
	text = TrimText(text, maxTokens*4) // 粗い文字上限（トークン近似）
	return text, "brave-search", nil
}
