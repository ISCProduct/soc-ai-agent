package companyfetch

import (
	"Backend/internal/openai"
	"context"
	"fmt"
	"strings"
)

// LLM は企業情報パイプライン用の OpenAI ラッパ。
type LLM struct {
	Client *openai.Client
	Search CompanySearchProvider // 未設定時は OpenAI Search Lite
	Budget SearchBudget          // 任意。設定時は Search 前に月次予算を検査する
}

// NewLLM は Client と env 由来の Search Provider を束ねる。
// 予算ガードは SetSearchBudget / LLM.Budget で後から注入する。
func NewLLM(client *openai.Client) *LLM {
	return &LLM{
		Client: client,
		Search: NewSearchProviderFromEnv(client),
	}
}

// ExtractJSON は与えられたテキスト前提のプロンプトから JSON を抽出する（Extract モデル）。
func (l *LLM) ExtractJSON(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (text string, model string, err error) {
	if l == nil || l.Client == nil {
		return "", "", fmt.Errorf("openai client is nil")
	}
	if maxTokens <= 0 {
		maxTokens = 600
	}
	model = ExtractModel()
	text, err = l.Client.ChatCompletionJSON(ctx, systemPrompt, userPrompt, 0.2, maxTokens, model)
	return text, model, err
}

// SearchLiteJSON は月次予算チェック後、CompanySearchProvider 経由で検索する（既定: OpenAI Search Lite）。
func (l *LLM) SearchLiteJSON(ctx context.Context, userPrompt string, maxTokens int) (text string, model string, err error) {
	if l == nil {
		return "", "", fmt.Errorf("openai client is nil")
	}
	if l.Budget != nil {
		if err := l.Budget.AllowSearch(); err != nil {
			return "", "", err
		}
	}
	if maxTokens <= 0 {
		maxTokens = 1200
	}
	provider := l.ensureProvider()
	if provider == nil {
		return "", "", fmt.Errorf("search provider is nil")
	}
	return provider.Search(ctx, userPrompt, maxTokens)
}

// SearchJSON は互換のため残す。企業情報パイプラインでは SearchLiteJSON を使うこと。
func (l *LLM) SearchJSON(ctx context.Context, userPrompt string, maxTokens int) (text string, model string, err error) {
	return l.SearchLiteJSON(ctx, userPrompt, maxTokens)
}

// SearchLiteThenParse は安価 Search → Parse の 2 段構成（高額な deep search なし）。
func (l *LLM) SearchLiteThenParse(ctx context.Context, searchPrompt, parseSystem, parseUserPrefix string, parseMaxTokens int) (jsonText string, modelsUsed string, err error) {
	searchText, searchModel, err := l.SearchLiteJSON(ctx, searchPrompt, 1500)
	if err != nil {
		return "", "", fmt.Errorf("web search failed: %w", err)
	}
	if strings.TrimSpace(searchText) == "" {
		return "", "", fmt.Errorf("web search returned empty")
	}

	parseUser := parseUserPrefix + "\n\n---\n検索結果:\n" + TrimText(searchText, 2000)
	parsed, parseModel, err := l.ParseJSON(ctx, parseSystem, parseUser, parseMaxTokens)
	if err != nil {
		return "", searchModel, err
	}
	return parsed, searchModel + "+" + parseModel, nil
}

// SearchThenParse は SearchLiteThenParse のエイリアス（deep search は使わない）。
func (l *LLM) SearchThenParse(ctx context.Context, searchPrompt, parseSystem, parseUserPrefix string, parseMaxTokens int) (jsonText string, modelsUsed string, err error) {
	return l.SearchLiteThenParse(ctx, searchPrompt, parseSystem, parseUserPrefix, parseMaxTokens)
}

// ParseJSON は Parse モデルで JSON 化する。
func (l *LLM) ParseJSON(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (text string, model string, err error) {
	if l == nil || l.Client == nil {
		return "", "", fmt.Errorf("openai client is nil")
	}
	if maxTokens <= 0 {
		maxTokens = 600
	}
	model = ParseModel()
	text, err = l.Client.ChatCompletionJSON(ctx, systemPrompt, userPrompt, 0.2, maxTokens, model)
	return text, model, err
}

// ExtractJSONObject はレスポンス文字列から最初の JSON オブジェクト部分を切り出す。
func ExtractJSONObject(text string) (string, error) {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return "", fmt.Errorf("no valid JSON object found")
	}
	return text[start : end+1], nil
}
