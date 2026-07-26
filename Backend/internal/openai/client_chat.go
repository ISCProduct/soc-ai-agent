package openai

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func (cli *Client) ChatCompletionJSON(ctx context.Context, systemPrompt, userPrompt string, temperature float32, maxTokens int, modelOverride ...string) (string, error) {
	if cli == nil || cli.c == nil {
		return "", errors.New("openai client is nil")
	}

	model := cli.DefaultModel
	if len(modelOverride) > 0 && modelOverride[0] != "" {
		model = modelOverride[0]
	}
	if strings.TrimSpace(model) == "" {
		model = "gpt-4o-mini"
	}

	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		ctxReq, cancel := context.WithTimeout(ctx, 60*time.Second)
		req := openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: userPrompt,
				},
			},
			Temperature:         temperature,
			MaxTokens:           0,
			MaxCompletionTokens: maxTokens,
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
		}

		resp, err := cli.c.CreateChatCompletion(ctxReq, req)
		if err != nil && isUnsupportedResponseFormatErr(err) {
			req.ResponseFormat = nil
			resp, err = cli.c.CreateChatCompletion(ctxReq, req)
		}
		if err != nil && isBetaLimitationsErr(err) {
			req.Temperature = 1
			resp, err = cli.c.CreateChatCompletion(ctxReq, req)
		}
		if err != nil && isModelNotFoundErr(err) && (len(modelOverride) == 0 || modelOverride[0] == "") && model != "gpt-4o-mini" {
			req.Model = "gpt-4o-mini"
			resp, err = cli.c.CreateChatCompletion(ctxReq, req)
		}
		cancel()

		if err == nil && len(resp.Choices) > 0 {
			content := strings.TrimSpace(resp.Choices[0].Message.Content)
			if content != "" {
				if cli.OnUsage != nil {
					cli.OnUsage(req.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
				}
				// キャッシュヒットのログを出力（存在すれば）
				if resp.Usage.PromptTokensDetails != nil {
					cached := resp.Usage.PromptTokensDetails.CachedTokens
					var hit float64
					if resp.Usage.PromptTokens > 0 {
						hit = float64(cached) / float64(resp.Usage.PromptTokens)
					}
					if cached > 0 && resp.Usage.PromptTokens > 0 {
						log.Printf("[openai] model=%s OpenAIプロンプトキャッシュ: 見込まれる改善率=%.2f%% キャッシュ利用=%d/%d ヒット率=%.2f", req.Model, hit*100, cached, resp.Usage.PromptTokens, hit)
					} else {
						log.Printf("[openai] model=%s OpenAIプロンプトキャッシュ: 見込まれる改善率=0.00%% キャッシュ利用=%d/%d ヒット率=0.00", req.Model, cached, resp.Usage.PromptTokens)
					}
					// JSON structured log for machines
					if jl, err := json.Marshal(map[string]any{
						"event":          "openai_prompt_cache",
						"model":          req.Model,
						"cached_tokens":  cached,
						"input_tokens":   resp.Usage.PromptTokens,
						"cache_hit_rate": hit,
					}); err == nil {
						log.Println(string(jl))
					}
					// prometheus metrics disabled in this environment
				}
				return content, nil
			}
			lastErr = errors.New("empty response from model")
		} else if err != nil {
			lastErr = err
		}

		backoff := time.Duration(1<<attempt) * time.Second
		jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
		time.Sleep(backoff + jitter)
	}

	if lastErr == nil {
		lastErr = errors.New("no response from model")
	}
	return "", lastErr
}

// WebSearchJSON は OpenAI Web Search モデルで 1 クエリだけ実行し、短い JSON テキストを返す。
// 企業実在確認などトークンを抑えた検証用途向け。RAG の多段調査パイプラインは使わない。
func (cli *Client) WebSearchJSON(ctx context.Context, userPrompt string, maxTokens int, modelOverride ...string) (string, error) {
	if cli == nil || cli.c == nil {
		return "", errors.New("openai client is nil")
	}
	if maxTokens <= 0 {
		maxTokens = 200
	}

	model := os.Getenv("OPENAI_WEB_SEARCH_MODEL")
	if len(modelOverride) > 0 && modelOverride[0] != "" {
		model = modelOverride[0]
	}
	if strings.TrimSpace(model) == "" {
		model = "gpt-4o-search-preview"
	}

	ctxReq, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		MaxTokens:           0,
		MaxCompletionTokens: maxTokens,
	}

	resp, err := cli.c.CreateChatCompletion(ctxReq, req)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("no response from web search model")
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if cli.OnUsage != nil {
		cli.OnUsage(model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	}
	log.Printf("[openai] web_search model=%s prompt_tokens=%d completion_tokens=%d",
		model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	return content, nil
}

// ResponsesWithMaxTokens は Responses API を使い、maxOutputTokens を指定してテキストを取得します。
