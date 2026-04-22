package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// UsageHook はAPIコール成功時に呼ばれるコールバック。
// model: 使用モデル名, promptTokens: 入力トークン数, completionTokens: 出力トークン数
type UsageHook func(model string, promptTokens, completionTokens int)

// Client は go-openai SDK をラップします。
type Client struct {
	c            *openai.Client
	DefaultModel string
	apiKey       string
	baseURL      string
	OnUsage      UsageHook // オプション: コール成功時にトークン使用量を通知
}

func init() {
	// ジッター用の乱数初期化
	rand.Seed(time.Now().UnixNano())
}

func NewFromEnv(optionalModel string) (*Client, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil, errors.New("OPENAI_API_KEY is not set")
	}

	model := optionalModel
	if model == "" {
		model = os.Getenv("OPENAI_MODEL")
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	cli := openai.NewClient(key)
	return &Client{c: cli, DefaultModel: model, apiKey: key, baseURL: "https://api.openai.com/v1"}, nil
}

// NewWithBaseURL はテスト用コンストラクタ。baseURL を差し替えてモックサーバーを利用できる。
func NewWithBaseURL(baseURL, model string) *Client {
	config := openai.DefaultConfig("test-key")
	config.BaseURL = baseURL
	return &Client{c: openai.NewClientWithConfig(config), DefaultModel: model, apiKey: "test-key", baseURL: baseURL}
}

func (cli *Client) BaseURL() string {
	if cli.baseURL != "" {
		return cli.baseURL
	}
	return "https://api.openai.com/v1"
}

func (cli *Client) callResponsesAPI(ctx context.Context, input interface{}, model string, temperature *float32, maxOutputTokens int, includeTextFormat bool) (string, error) {
	if cli.apiKey == "" {
		return "", errors.New("openai api key is not set")
	}

	type responsesRequest struct {
		Model           string      `json:"model"`
		Input           interface{} `json:"input"`
		MaxOutputTokens int         `json:"max_output_tokens,omitempty"`
		Temperature     *float32    `json:"temperature,omitempty"`
		Text            interface{} `json:"text,omitempty"`
		Reasoning       interface{} `json:"reasoning,omitempty"`
	}

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
		payload.Text = map[string]interface{}{
			"format": map[string]string{
				"type": "text",
			},
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cli.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
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
		return "", errors.New(string(respBody))
	}

	type responsesContent struct {
		Type    string `json:"type"`
		Text    string `json:"text"`
		Refusal string `json:"refusal"`
	}
	type responsesOutput struct {
		Content []responsesContent `json:"content"`
	}
	type responsesUsage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
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
		cli.OnUsage(model, parsed.Usage.InputTokens, parsed.Usage.OutputTokens)
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

func (cli *Client) callResponsesAPIWithTempFallback(ctx context.Context, input interface{}, model string, temperature *float32, maxOutputTokens int, includeTextFormat bool) (string, error) {
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
		messageInput := []map[string]interface{}{
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
			content, err = cli.callResponsesAPI(ctxReq, messageInput, model, nil, 600, false)
		}
		if err != nil && strings.Contains(err.Error(), "empty response from responses api") {
			content, err = cli.callResponsesAPI(ctxReq, input, model, nil, 600, false)
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
	return resp.Data[0].Embedding, nil
}

// ResponsesWithTemperature は system と user プロンプトを分けて、温度パラメータ付きでリクエストします
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
		messageInput := []map[string]interface{}{
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
			content, err = cli.callResponsesAPIWithTempFallback(ctxReq, messageInput, model, &temperature, 100, false)
		}
		if err != nil && strings.Contains(err.Error(), "empty response from responses api") {
			combinedPrompt := strings.TrimSpace(systemPrompt)
			if combinedPrompt != "" {
				combinedPrompt += "\n\n"
			}
			combinedPrompt += userPrompt
			content, err = cli.callResponsesAPIWithTempFallback(ctxReq, combinedPrompt, model, &temperature, 100, false)
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

// WebSearchQuery は Responses API の web_search_preview ツールを使ってWeb検索を実行し、結果テキストを返します。
func (cli *Client) WebSearchQuery(ctx context.Context, query string) (string, error) {
	if cli.apiKey == "" {
		return "", errors.New("openai api key is not set")
	}

	type webSearchTool struct {
		Type string `json:"type"`
	}
	type webSearchRequest struct {
		Model string          `json:"model"`
		Tools []webSearchTool `json:"tools"`
		Input string          `json:"input"`
	}

	model := cli.DefaultModel
	if model == "" {
		model = "gpt-4o-search-preview"
	}
	payload := webSearchRequest{
		Model: model,
		Tools: []webSearchTool{{Type: "web_search_preview"}},
		Input: query,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cli.apiKey)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.New(string(respBody))
	}

	type outputContent struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type outputItem struct {
		Type    string          `json:"type"`
		Content []outputContent `json:"content"`
	}
	type webSearchResponse struct {
		Output     []outputItem `json:"output"`
		OutputText string       `json:"output_text"`
	}

	var parsed webSearchResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if strings.TrimSpace(parsed.OutputText) != "" {
		return strings.TrimSpace(parsed.OutputText), nil
	}
	for _, out := range parsed.Output {
		for _, c := range out.Content {
			if strings.TrimSpace(c.Text) != "" {
				return strings.TrimSpace(c.Text), nil
			}
		}
	}
	return "", errors.New("empty web search response")
}

// ResponsesWithMaxTokens は Responses API を使い、maxOutputTokens を指定してテキストを取得します。
// Chat Completions API の代替として、JSON レスポンスが必要な場合にも利用できます。
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
		messageInput := []map[string]interface{}{
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
