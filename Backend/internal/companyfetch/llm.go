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

// SearchJSON は Search モデルで短文 JSON/テキストを取得する。
func (l *LLM) SearchJSON(ctx context.Context, userPrompt string, maxTokens int) (text string, model string, err error) {
	if l == nil || l.Client == nil {
		return "", "", fmt.Errorf("openai client is nil")
	}
	if maxTokens <= 0 {
		maxTokens = 1200
	}
	model = SearchModel()
	text, err = l.Client.WebSearchJSON(ctx, userPrompt, maxTokens, model)
	if err != nil || strings.TrimSpace(text) == "" {
		// mini-search 不足時のみ deep search
		deep := DeepSearchModel()
		if deep != model {
			text, err = l.Client.WebSearchJSON(ctx, userPrompt, maxTokens, deep)
			model = deep
		}
	}
	return text, model, err
}

// SearchThenParse は Search → Parse の 2 段構成。
func (l *LLM) SearchThenParse(ctx context.Context, searchPrompt, parseSystem, parseUserPrefix string, parseMaxTokens int) (jsonText string, modelsUsed string, err error) {
	searchText, searchModel, err := l.SearchJSON(ctx, searchPrompt, 1500)
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
