package openai

import (
	"context"
	"errors"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

func (cli *Client) Embedding(ctx context.Context, input string, modelOverride ...string) ([]float32, error) {
	if cli == nil || cli.c == nil {
		return nil, errors.New("openai client is nil")
	}
	if strings.TrimSpace(input) == "" {
		return nil, errors.New("embedding input is empty")
	}

	model := os.Getenv("OPENAI_EMBEDDING_MODEL")
	if len(modelOverride) > 0 && strings.TrimSpace(modelOverride[0]) != "" {
		model = modelOverride[0]
	}
	if strings.TrimSpace(model) == "" {
		model = "text-embedding-3-small"
	}

	resp, err := cli.c.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(model),
		Input: []string{input},
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, errors.New("empty embedding response")
	}
	if cli.OnUsage != nil && resp.Usage.PromptTokens > 0 {
		cli.OnUsage(model, resp.Usage.PromptTokens, 0)
	}
	return resp.Data[0].Embedding, nil
}
