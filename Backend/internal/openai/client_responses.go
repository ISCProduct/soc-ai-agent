package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

type responsesRequest struct {
	Model           string           `json:"model"`
	Input           any              `json:"input"`
	MaxOutputTokens int              `json:"max_output_tokens,omitempty"`
	Temperature     *float32         `json:"temperature,omitempty"`
	Text            any              `json:"text,omitempty"`
	Reasoning       any              `json:"reasoning,omitempty"`
	Tools           []map[string]any `json:"tools,omitempty"`
	ToolChoice      any              `json:"tool_choice,omitempty"`
}

func (cli *Client) callResponsesAPI(ctx context.Context, input any, model string, temperature *float32, maxOutputTokens int, includeTextFormat bool) (string, error) {
	payload := responsesRequest{
		Model:           model,
		Input:           input,
		MaxOutputTokens: maxOutputTokens,
		Temperature:     temperature,
	}
	// reasoning.effort は o1/o3 系の推論モデルのみサポート。
	// gpt-4o-mini 等の標準モデルで送ると unsupported_parameter エラーになる。
	if isReasoningModel(model) {
		payload.Reasoning = map[string]string{"effort": "low"}
	}
	if includeTextFormat {
		payload.Text = map[string]any{
			"format": map[string]string{
				"type": "text",
			},
		}
	}
	return cli.doResponses(ctx, payload)
}

// ResponsesAPIError は Responses API が 2xx 以外を返したときのエラー。
// ステータスコードを保持することで、本文が空でもエラー内容が判別でき、
// 429 / 5xx の再試行判定（isRetryableAPIErr）も機能する。
type ResponsesAPIError struct {
	StatusCode int
	Body       string
}

func (e *ResponsesAPIError) Error() string {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return fmt.Sprintf("responses API がステータス %d を返しました（本文なし）", e.StatusCode)
	}
	return fmt.Sprintf("responses API がステータス %d を返しました: %s", e.StatusCode, body)
}

// Retryable は 429 / 5xx を再試行可能とみなす。
func (e *ResponsesAPIError) Retryable() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= 500
}

func (cli *Client) doResponses(ctx context.Context, payload responsesRequest) (string, error) {
	if cli.apiKey == "" {
		return "", errors.New("openai api key is not set")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cli.baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cli.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &ResponsesAPIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	type responsesContent struct {
		Type    string `json:"type"`
		Text    string `json:"text"`
		Refusal string `json:"refusal"`
	}
	type responsesOutput struct {
		Content []responsesContent `json:"content"`
	}
	type promptTokensDetails struct {
		CachedTokens int `json:"cached_tokens,omitempty"`
	}
	type responsesUsage struct {
		InputTokens         int                  `json:"input_tokens"`
		OutputTokens        int                  `json:"output_tokens"`
		PromptTokensDetails *promptTokensDetails `json:"prompt_tokens_details,omitempty"`
	}
	type responsesResponse struct {
		Output            []responsesOutput `json:"output"`
		OutputText        string            `json:"output_text"`
		Usage             responsesUsage    `json:"usage"`
		IncompleteDetails struct {
			Reason string `json:"reason"`
		} `json:"incomplete_details"`
	}
	type responsesError struct {
		Message string `json:"message"`
	}
	type responsesErrorWrapper struct {
		Error responsesError `json:"error"`
	}

	var parsed responsesResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if cli.OnUsage != nil && (parsed.Usage.InputTokens > 0 || parsed.Usage.OutputTokens > 0) {
		cli.OnUsage(payload.Model, parsed.Usage.InputTokens, parsed.Usage.OutputTokens)
		if parsed.Usage.PromptTokensDetails != nil {
			cached := parsed.Usage.PromptTokensDetails.CachedTokens
			var hit float64
			if parsed.Usage.InputTokens > 0 {
				hit = float64(cached) / float64(parsed.Usage.InputTokens)
			}
			// 日本語プレーンテキストログ: 見込まれる改善率、キャッシュ利用、ヒット率
			if cached > 0 && parsed.Usage.InputTokens > 0 {
				log.Printf("[openai] model=%s OpenAIプロンプトキャッシュ: 見込まれる改善率=%.2f%% キャッシュ利用=%d/%d ヒット率=%.2f", payload.Model, hit*100, cached, parsed.Usage.InputTokens, hit)
			} else {
				log.Printf("[openai] model=%s OpenAIプロンプトキャッシュ: 見込まれる改善率=0.00%% キャッシュ利用=%d/%d ヒット率=0.00", payload.Model, cached, parsed.Usage.InputTokens)
			}
			// JSON structured log for machines
			if jl, err := json.Marshal(map[string]any{
				"event":          "openai_prompt_cache",
				"model":          payload.Model,
				"cached_tokens":  cached,
				"input_tokens":   parsed.Usage.InputTokens,
				"cache_hit_rate": hit,
			}); err == nil {
				log.Println(string(jl))
			}
			// prometheus metrics disabled in this environment
		}
	}
	if strings.TrimSpace(parsed.OutputText) != "" {
		return strings.TrimSpace(parsed.OutputText), nil
	}
	var parsedErr responsesErrorWrapper
	if err := json.Unmarshal(respBody, &parsedErr); err == nil {
		if strings.TrimSpace(parsedErr.Error.Message) != "" {
			return "", errors.New(parsedErr.Error.Message)
		}
	}

	var parts []string
	for _, out := range parsed.Output {
		for _, c := range out.Content {
			if strings.TrimSpace(c.Text) != "" {
				parts = append(parts, strings.TrimSpace(c.Text))
				continue
			}
			if strings.TrimSpace(c.Refusal) != "" {
				return "", errors.New(strings.TrimSpace(c.Refusal))
			}
		}
	}
	if len(parts) == 0 {
		if strings.TrimSpace(parsed.IncompleteDetails.Reason) != "" {
			return "", errors.New("empty response from responses api: " + parsed.IncompleteDetails.Reason)
		}
		snippet := strings.TrimSpace(string(respBody))
		if len(snippet) > 1000 {
			snippet = snippet[:1000]
		}
		return "", errors.New("empty response from responses api: " + snippet)
	}
	return strings.Join(parts, "\n"), nil
}

// isReasoningModel は o1/o3 系の推論モデルかどうかを判定します。
// reasoning.effort パラメータはこれらのモデルのみサポートされています。
func isReasoningModel(model string) bool {
	lower := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(lower, "o1") || strings.HasPrefix(lower, "o3") || strings.HasPrefix(lower, "o-")
}

func isUnsupportedTemperatureErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Unsupported parameter") && strings.Contains(msg, "temperature")
}

func isUnsupportedResponseFormatErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "response_format") && strings.Contains(msg, "Unsupported")
}

func isBetaLimitationsErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "beta-limitations") && strings.Contains(msg, "temperature")
}

func isModelNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "model") && (strings.Contains(msg, "not found") || strings.Contains(msg, "does not exist") || strings.Contains(msg, "unsupported"))
}

func (cli *Client) callResponsesAPIWithTempFallback(ctx context.Context, input any, model string, temperature *float32, maxOutputTokens int, includeTextFormat bool) (string, error) {
	content, err := cli.callResponsesAPI(ctx, input, model, temperature, maxOutputTokens, includeTextFormat)
	if err != nil && isUnsupportedTemperatureErr(err) {
		return cli.callResponsesAPI(ctx, input, model, nil, maxOutputTokens, includeTextFormat)
	}
	return content, err
}

func (cli *Client) Responses(ctx context.Context, input string, modelOverride ...string) (string, error) {
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
	// attempts を 5 回に増やし、各リクエストにタイムアウトを設定
	for attempt := 1; attempt <= 5; attempt++ {
		ctxReq, cancel := context.WithTimeout(ctx, 60*time.Second)
		messageInput := []map[string]any{
			{
				"role": "user",
				"content": []map[string]string{
					{
						"type": "input_text",
						"text": input,
					},
				},
			},
		}
		content, err := cli.callResponsesAPI(ctxReq, messageInput, model, nil, 600, true)
		if err != nil && strings.Contains(err.Error(), "empty response from responses api") {
			// キャッシュ活用のため、system/user を分離したまま再試行する（combinedPrompt を作らない）
			content, err = cli.callResponsesAPI(ctxReq, messageInput, model, nil, 600, false)
		}
		if err != nil && strings.Contains(err.Error(), "empty response from responses api") {
			// それでも空応答なら maxOutputTokens を増やして再試行（system/user を分離したまま）
			content, err = cli.callResponsesAPI(ctxReq, messageInput, model, nil, 1200, false)
		}
		if err != nil && strings.Contains(err.Error(), "max_output_tokens") {
			content, err = cli.callResponsesAPI(ctxReq, messageInput, model, nil, 1200, true)
		}
		cancel()

		if err == nil && strings.TrimSpace(content) != "" {
			return strings.TrimSpace(content), nil
		}
		if err == nil {
			lastErr = errors.New("empty response from model")
			println("OpenAI API empty content (attempt", attempt, ")")
		} else {
			lastErr = err
			println("OpenAI API error (attempt", attempt, "):", err.Error())
		}

		// 指数バックオフ + ジッター
		backoff := time.Duration(1<<attempt) * time.Second
		jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
		time.Sleep(backoff + jitter)
	}

	if lastErr == nil {
		lastErr = errors.New("no response from model")
	}
	return "", lastErr
}
func (cli *Client) ResponsesWithTemperature(ctx context.Context, systemPrompt, userPrompt string, temperature float32, modelOverride ...string) (string, error) {
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
		messageInput := []map[string]any{
			{
				"role": "system",
				"content": []map[string]string{
					{
						"type": "input_text",
						"text": systemPrompt,
					},
				},
			},
			{
				"role": "user",
				"content": []map[string]string{
					{
						"type": "input_text",
						"text": userPrompt,
					},
				},
			},
		}
		content, err := cli.callResponsesAPIWithTempFallback(ctxReq, messageInput, model, &temperature, 100, true)
		if err != nil && strings.Contains(err.Error(), "empty response from responses api") {
			// キャッシュを活かすために system/user を分離したまま再試行
			content, err = cli.callResponsesAPIWithTempFallback(ctxReq, messageInput, model, &temperature, 100, false)
		}
		if err != nil && strings.Contains(err.Error(), "empty response from responses api") {
			// 空応答が続く場合は出力トークン上限を増やして再試行
			content, err = cli.callResponsesAPIWithTempFallback(ctxReq, messageInput, model, &temperature, 200, false)
		}
		if err != nil && strings.Contains(err.Error(), "max_output_tokens") {
			content, err = cli.callResponsesAPIWithTempFallback(ctxReq, messageInput, model, &temperature, 200, true)
		}
		cancel()

		if err == nil && strings.TrimSpace(content) != "" {
			return strings.TrimSpace(content), nil
		}
		if err == nil {
			lastErr = errors.New("empty response from model")
			println("OpenAI API empty content (attempt", attempt, ")")
		} else {
			lastErr = err
			println("OpenAI API error (attempt", attempt, "):", err.Error())
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

// ChatCompletionJSON uses the go-openai SDK to request a JSON response.
func (cli *Client) ResponsesWithMaxTokens(ctx context.Context, systemPrompt, userPrompt string, temperature float32, maxOutputTokens int, modelOverride ...string) (string, error) {
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
		ctxReq, cancel := context.WithTimeout(ctx, 90*time.Second)
		messageInput := []map[string]any{
			{
				"role": "system",
				"content": []map[string]string{
					{"type": "input_text", "text": systemPrompt},
				},
			},
			{
				"role": "user",
				"content": []map[string]string{
					{"type": "input_text", "text": userPrompt},
				},
			},
		}
		content, err := cli.callResponsesAPIWithTempFallback(ctxReq, messageInput, model, &temperature, maxOutputTokens, false)
		if err != nil && strings.Contains(err.Error(), "empty response from responses api") {
			content, err = cli.callResponsesAPIWithTempFallback(ctxReq, messageInput, model, &temperature, maxOutputTokens, true)
		}
		if err != nil && strings.Contains(err.Error(), "max_output_tokens") {
			content, err = cli.callResponsesAPIWithTempFallback(ctxReq, messageInput, model, &temperature, maxOutputTokens*2, false)
		}
		cancel()

		if err == nil && strings.TrimSpace(content) != "" {
			return strings.TrimSpace(content), nil
		}
		if err == nil {
			lastErr = errors.New("empty response from model")
		} else {
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
