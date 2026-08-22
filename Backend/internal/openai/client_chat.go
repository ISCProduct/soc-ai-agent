package openai

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// Responses API の web_search + 安価モデル。Chat Completions の search-preview / search-api は使わない。
const defaultWebSearchModel = "gpt-4o-mini"

func isChatCompletionsSearchModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(m, "search-preview") || strings.Contains(m, "search-api")
}

func resolveWebSearchModel(override string) string {
	for _, m := range []string{
		override,
		os.Getenv("OPENAI_WEB_SEARCH_MODEL"),
		os.Getenv("OPENAI_COMPANY_SEARCH_MODEL"),
		defaultWebSearchModel,
	} {
		m = strings.TrimSpace(m)
		if m == "" || isChatCompletionsSearchModel(m) {
			continue
		}
		return m
	}
	return defaultWebSearchModel
}

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

// isRetryableAPIErr は 429 / 5xx など再試行すれば成功しうるエラーか判定する。
func isRetryableAPIErr(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return apiErr.HTTPStatusCode == http.StatusTooManyRequests || apiErr.HTTPStatusCode >= 500
	}
	return false
}

// WebSearchJSON は Responses API の web_search で 1 クエリだけ実行する。
// Chat Completions の search-preview / gpt-5-search-api は使わない（高トークン・高額）。
func (cli *Client) WebSearchJSON(ctx context.Context, userPrompt string, maxTokens int, modelOverride ...string) (string, error) {
	if cli == nil || cli.c == nil {
		return "", errors.New("openai client is nil")
	}
	if maxTokens < 600 {
		maxTokens = 600
	}
	requested := ""
	if len(modelOverride) > 0 {
		requested = modelOverride[0]
	}
	model := resolveWebSearchModel(requested)
	if requested != "" && isChatCompletionsSearchModel(requested) {
		log.Printf("[openai] web_search ignore retired model=%s use=%s", requested, model)
	}

	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		started := time.Now()
		ctxReq, cancel := context.WithTimeout(ctx, 45*time.Second)
		text, err := cli.doResponses(ctxReq, responsesRequest{
			Model:           model,
			Input:           userPrompt,
			MaxOutputTokens: maxTokens,
			Tools: []map[string]any{{
				"type":                "web_search",
				"search_context_size": "high",
			}},
			ToolChoice: "required",
		})
		elapsedMs := time.Since(started).Milliseconds()
		cancel()
		if err == nil && strings.TrimSpace(text) != "" {
			log.Printf("[openai] web_search model=%s elapsed_ms=%d attempt=%d", model, elapsedMs, attempt)
			return strings.TrimSpace(text), nil
		}
		if err != nil {
			lastErr = err
			if !isRetryableAPIErr(err) && !strings.Contains(strings.ToLower(err.Error()), "empty response") {
				return "", err
			}
		} else {
			lastErr = errors.New("empty response from web search")
		}
		if attempt == maxAttempts {
			break
		}
		backoff := time.Duration(1<<attempt) * time.Second
		jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(backoff + jitter):
		}
	}
	if lastErr == nil {
		lastErr = errors.New("no response from web search")
	}
	return "", lastErr
}

// ResponsesWithMaxTokens は Responses API を使い、maxOutputTokens を指定してテキストを取得します。
