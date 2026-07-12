package companyfetch

import (
	"Backend/internal/openai"
	"context"
	"fmt"
)

// OpenAISearchProvider は既存の OpenAI Web Search（Search Lite）実装。
type OpenAISearchProvider struct {
	Client *openai.Client
	Model  string
}

func NewOpenAISearchProvider(client *openai.Client) *OpenAISearchProvider {
	return &OpenAISearchProvider{Client: client, Model: SearchModel()}
}

func (p *OpenAISearchProvider) Name() string { return "openai" }

func (p *OpenAISearchProvider) Search(ctx context.Context, query string, maxTokens int) (string, string, error) {
	if p == nil || p.Client == nil {
		return "", "", fmt.Errorf("openai search provider: client is nil")
	}
	if maxTokens <= 0 {
		maxTokens = 1200
	}
	model := p.Model
	if model == "" {
		model = SearchModel()
	}
	text, err := p.Client.WebSearchJSON(ctx, query, maxTokens, model)
	if err != nil {
		return "", model, providerSearchError("openai", err)
	}
	return text, model, nil
}
