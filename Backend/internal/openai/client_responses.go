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
	"slices"
	"strings"
	"sync/atomic"
	"time"
)

// truncationFlagKey は「出力が max_output_tokens で切れたか」を呼び出し側へ
// 伝えるためのコンテキストキー（#1529）。doResponses が書き、呼び出し側が読む。
//
// 戻り値で返さないのは、Responses API の呼び出しが
// doResponses → callResponsesAPI → …WithTempFallback → 各公開メソッド と4段あり、
// 全段の戻り値を増やすと12箇所の呼び出し元を巻き込むため。
// フォールバック判定（withFallbackFlag）と同じ形に揃えている。
type truncationFlagKey struct{}

// WithTruncationFlag は上限到達フラグをコンテキストに載せる。
//
// 切れた出力を「壊れていない出力」として扱いたくない呼び出し側だけが載せる。
// 載せない呼び出し側の挙動は変わらない（従来どおり途中までの本文を受け取る）。
//
// フラグが表すのは「最後の応答が切れていたか」である。公開メソッドは
// 空応答・上限エラーのときに条件を変えて呼び直すので、立ったままにすると
// 「1回目が上限に当たり、出力枠を倍にした2回目が成功した」場合に
// 正常な応答を切れた扱いにしてしまう。
//
// したがって1つのフラグ付きコンテキストは1回の論理呼び出しで使い切ること。
// 並行する複数の呼び出しで共有すると「最後の応答」がどれか決まらない。
func WithTruncationFlag(ctx context.Context) context.Context {
	return context.WithValue(ctx, truncationFlagKey{}, &atomic.Bool{})
}

// OutputTruncated は WithTruncationFlag を通したコンテキストで、
// 最後の応答が max_output_tokens により途中で切れたかを返す。
func OutputTruncated(ctx context.Context) bool {
	flag, ok := ctx.Value(truncationFlagKey{}).(*atomic.Bool)
	return ok && flag.Load()
}

// setOutputTruncated は今回の応答が上限到達だったかをコンテキストへ記録する。
func setOutputTruncated(ctx context.Context, truncated bool) {
	if flag, ok := ctx.Value(truncationFlagKey{}).(*atomic.Bool); ok {
		flag.Store(truncated)
	}
}

// truncationReasonMaxTokens は Responses API が返す上限到達の理由。
const truncationReasonMaxTokens = "max_output_tokens"

// Responses API の text.format.type に入れる値。
//
// textFormatNone は text を送らない（Chat Completions の response_format 未指定に相当）。
// textFormatJSON は JSON mode。指定すると本文が素の JSON になり、コードフェンスや
// 前置きが付かなくなる（#1583）。Chat Completions の
// response_format={"type":"json_object"} と同じ強制で、プロンプト側に "JSON" の語が
// 必要な点も同じ。
const (
	textFormatNone = ""
	textFormatText = "text"
	textFormatJSON = "json_object"
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

// countWebSearchTools はリクエストに含まれる web_search ツールの数を返す。
//
// web_search は検索結果の固定トークンに加えて1コール単位のツール料が課金される。
// モデル名からは判別できない（同じ gpt-4o-mini でも通常のチャットと混ざる）ため、
// リクエスト内容から数える。
func countWebSearchTools(tools []map[string]any) int {
	n := 0
	for _, t := range tools {
		if v, ok := t["type"].(string); ok && strings.HasPrefix(v, "web_search") {
			n++
		}
	}
	return n
}

func (cli *Client) callResponsesAPI(ctx context.Context, input any, model string, temperature *float32, maxOutputTokens int, textFormat string) (string, error) {
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
	if textFormat != textFormatNone {
		payload.Text = map[string]any{
			"format": map[string]string{
				"type": textFormat,
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
	ctx = withFallbackFlag(ctx)
	// ensureText と二重管理にすると local プロバイダで Responses 系が全滅するため、
	// ここも同じガードに寄せる（#1293 レビュー指摘）
	if err := cli.ensureText(); err != nil {
		return "", err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cli.BaseURL()+"/responses", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cli.textKey)
	req.Header.Set("Content-Type", "application/json")

	client := cli.httpClientFor("text", 120*time.Second)
	start := time.Now()
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
	// 上限到達は本文が途中まで返っていても起きる。OutputText の早期 return より
	// 先に記録しないと、切れた本文が正常な応答として呼び出し側へ渡る（#1529）。
	// 切れていなければ下げる（出力枠を増やした再試行が成功した場合に効く）。
	setOutputTruncated(ctx, parsed.IncompleteDetails.Reason == truncationReasonMaxTokens)
	if parsed.Usage.InputTokens > 0 || parsed.Usage.OutputTokens > 0 {
		cli.reportUsage(ctx, usageReport{
			provider:         cli.textProvider,
			model:            payload.Model,
			promptTokens:     parsed.Usage.InputTokens,
			completionTokens: parsed.Usage.OutputTokens,
			latency:          time.Since(start),
			cacheHit:         parsed.Usage.PromptTokensDetails != nil && parsed.Usage.PromptTokensDetails.CachedTokens > 0,
			webSearchCalls:   countWebSearchTools(payload.Tools),
		})
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

// isUnsupportedJSONModeErr は JSON mode（text.format.type=json_object）そのものが
// 400 で拒否されたかを判定する（#1595）。
//
// 400 は同じ条件で投げ直しても結果が変わらないため、これに当たったら外側の
// リトライへ渡さず、形式を落として1度だけやり直す。
//
// ただし 400 には JSON mode 以外の原因（プロンプト長超過・必須パラメータ不足・
// コンテンツフィルタ）も含まれる。それらで形式を落としても無意味なので、
// **JSON mode 起因と読めるものだけ**に絞る。判定できない 400 は従来どおり
// 外側のループへ渡す。
//
// 判定語は実際に OpenAI が返す文面に合わせている。
//   - "Response input messages must contain the word 'json' in some form to use
//     'text.format' of type 'json_object'."（プロンプトに json の語が無い / #1595 で実測）
//   - "Unsupported parameter: 'text.format' is not supported with this model."
//     （既存の isUnsupportedTemperatureErr が拾う文面と同型）
//   - "Unknown parameter: 'text.format'."（パラメータ自体を知らないエンドポイント）
//   - "Invalid value: 'json_object'. Supported values are: ..."（値だけ非対応）
//   - OpenAI 互換実装の "response_format json_object is not supported" 系
func isUnsupportedJSONModeErr(err error) bool {
	var apiErr *ResponsesAPIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		return false
	}
	body := strings.ToLower(apiErr.Body)
	// 出力形式のパラメータ名・値に触れていない 400 は対象外
	// （プロンプト長超過やコンテンツフィルタをここで拾わないため）。
	if !strings.Contains(body, "text.format") &&
		!strings.Contains(body, "json_object") &&
		!strings.Contains(body, "response_format") {
		return false
	}
	return slices.ContainsFunc([]string{
		"unsupported",
		"not supported",
		"unknown parameter",
		"invalid value",
		"must contain the word",
	}, func(marker string) bool { return strings.Contains(body, marker) })
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

func (cli *Client) callResponsesAPIWithTempFallback(ctx context.Context, input any, model string, temperature *float32, maxOutputTokens int, textFormat string) (string, error) {
	content, err := cli.callResponsesAPI(ctx, input, model, temperature, maxOutputTokens, textFormat)
	if err != nil && isUnsupportedTemperatureErr(err) {
		return cli.callResponsesAPI(ctx, input, model, nil, maxOutputTokens, textFormat)
	}
	return content, err
}

func (cli *Client) Responses(ctx context.Context, input string, modelOverride ...string) (string, error) {
	if err := cli.ensureText(); err != nil {
		return "", err
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
		content, err := cli.callResponsesAPI(ctxReq, messageInput, model, nil, 600, textFormatText)
		if err != nil && strings.Contains(err.Error(), "empty response from responses api") {
			// キャッシュ活用のため、system/user を分離したまま再試行する（combinedPrompt を作らない）
			content, err = cli.callResponsesAPI(ctxReq, messageInput, model, nil, 600, textFormatNone)
		}
		if err != nil && strings.Contains(err.Error(), "empty response from responses api") {
			// それでも空応答なら maxOutputTokens を増やして再試行（system/user を分離したまま）
			content, err = cli.callResponsesAPI(ctxReq, messageInput, model, nil, 1200, textFormatNone)
		}
		if err != nil && strings.Contains(err.Error(), "max_output_tokens") {
			content, err = cli.callResponsesAPI(ctxReq, messageInput, model, nil, 1200, textFormatText)
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
	if err := cli.ensureText(); err != nil {
		return "", err
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
		content, err := cli.callResponsesAPIWithTempFallback(ctxReq, messageInput, model, &temperature, 100, textFormatText)
		if err != nil && strings.Contains(err.Error(), "empty response from responses api") {
			// キャッシュを活かすために system/user を分離したまま再試行
			content, err = cli.callResponsesAPIWithTempFallback(ctxReq, messageInput, model, &temperature, 100, textFormatNone)
		}
		if err != nil && strings.Contains(err.Error(), "empty response from responses api") {
			// 空応答が続く場合は出力トークン上限を増やして再試行
			content, err = cli.callResponsesAPIWithTempFallback(ctxReq, messageInput, model, &temperature, 200, textFormatNone)
		}
		if err != nil && strings.Contains(err.Error(), "max_output_tokens") {
			content, err = cli.callResponsesAPIWithTempFallback(ctxReq, messageInput, model, &temperature, 200, textFormatText)
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

// ResponsesWithMaxTokens は Responses API で出力上限を指定してテキストを取得する。
// 出力形式は強制しない（JSON が欲しいときは ResponsesJSONWithMaxTokens を使う）。
func (cli *Client) ResponsesWithMaxTokens(ctx context.Context, systemPrompt, userPrompt string, temperature float32, maxOutputTokens int, modelOverride ...string) (string, error) {
	return cli.responsesWithMaxTokens(ctx, systemPrompt, userPrompt, temperature, maxOutputTokens, false, modelOverride...)
}

// ResponsesJSONWithMaxTokens は JSON mode（text.format.type=json_object）で呼ぶ。
//
// 素の JSON だけが返るため、コードフェンスや前置きの除去が不要になり、
// 構文として壊れた JSON も返らなくなる（#1583）。
// JSON mode はオプトインにしてある。JSON を期待しない呼び出し
// （面接の質問プラン等）に強制すると出力そのものが変わってしまうため。
//
// 注意: JSON mode はプロンプト（system か user）に "JSON" の語が無いと
// API がエラーを返す。呼び出し側のプロンプトで担保すること。
func (cli *Client) ResponsesJSONWithMaxTokens(ctx context.Context, systemPrompt, userPrompt string, temperature float32, maxOutputTokens int, modelOverride ...string) (string, error) {
	return cli.responsesWithMaxTokens(ctx, systemPrompt, userPrompt, temperature, maxOutputTokens, true, modelOverride...)
}

func (cli *Client) responsesWithMaxTokens(ctx context.Context, systemPrompt, userPrompt string, temperature float32, maxOutputTokens int, jsonMode bool, modelOverride ...string) (string, error) {
	if err := cli.ensureText(); err != nil {
		return "", err
	}

	model := cli.DefaultModel
	if len(modelOverride) > 0 && modelOverride[0] != "" {
		model = modelOverride[0]
	}
	if strings.TrimSpace(model) == "" {
		model = "gpt-4o-mini"
	}

	format := textFormatNone
	if jsonMode {
		format = textFormatJSON
	}

	const maxAttempts = 5
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
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
		content, err := cli.callResponsesAPIWithTempFallback(ctxReq, messageInput, model, &temperature, maxOutputTokens, format)
		// JSON mode 自体が 400 で拒否されたら、形式を落として1度だけやり直す（#1595）。
		// 400 は投げ直しても同じ結果なので、外側のループに任せると5回とも同じ400を
		// 踏んでスリープ分だけ空転してから失敗する。
		//
		// format を戻さないのは、以降の attempt（429 等で再試行になった場合）で
		// 同じ400を踏み直さないため。jsonMode はそのままにしておくので、
		// 下の「空応答なら text へ付け替える」経路は退避後も無効のままになる。
		if err != nil && format == textFormatJSON && isUnsupportedJSONModeErr(err) {
			log.Printf("[openai] model=%s JSON mode が拒否されたため text.format なしで再試行する: %v", model, err)
			format = textFormatNone
			content, err = cli.callResponsesAPIWithTempFallback(ctxReq, messageInput, model, &temperature, maxOutputTokens, format)
		}
		// 空応答のときに text.format を付け替えて試すのは JSON mode 以外だけ。
		// JSON mode で text を落とすと出力形式の強制も一緒に消えるので、
		// 同じ形式のまま外側のループで再試行させる。
		if err != nil && !jsonMode && strings.Contains(err.Error(), "empty response from responses api") {
			content, err = cli.callResponsesAPIWithTempFallback(ctxReq, messageInput, model, &temperature, maxOutputTokens, textFormatText)
		}
		if err != nil && strings.Contains(err.Error(), "max_output_tokens") {
			content, err = cli.callResponsesAPIWithTempFallback(ctxReq, messageInput, model, &temperature, maxOutputTokens*2, format)
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

		// 最後の試行のあとに待つ意味は無い（待ってから失敗を返すだけ）。
		if attempt == maxAttempts {
			break
		}
		backoff := time.Duration(1<<attempt) * time.Second
		jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
		// 待っている間に呼び出し側が諦めたら即座に返す（WebSearchJSON と同じ形）。
		// 返すのは ctx.Err() ではなく直前のエラーで、原因が分かるようにする。
		select {
		case <-ctx.Done():
			return "", lastErr
		case <-time.After(backoff + jitter):
		}
	}

	if lastErr == nil {
		lastErr = errors.New("no response from model")
	}
	return "", lastErr
}
