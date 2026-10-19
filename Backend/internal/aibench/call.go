package aibench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// CallResult は LLM 1回の呼び出し結果。
type CallResult struct {
	Text             string
	PromptTokens     int
	CompletionTokens int
	LatencyMS        int64
	// Truncated は出力上限に到達して途中で切れたことを示す。
	Truncated bool
	Err       error
	// Fatal は設定の問題（認証・キー未設定）で、品質の測定になっていないことを示す。
	Fatal bool
	// Unmeasured は通信・レート制限・サーバ側エラー。測り直せば消えるので
	// 破損率とレイテンシから除外する。
	Unmeasured bool
}

// isUnmeasurable は「モデルの出力品質とは無関係で、測り直せば消える」ステータスか。
// 429 はレート制限、5xx は API 側の一時障害。
//
// 429 でも残高切れは測り直しても消えないので、body を見て分ける
// （quotaExhausted / #1634）。status だけで判定しないこと。
func isUnmeasurable(status int, body []byte) bool {
	if status == http.StatusTooManyRequests {
		return !quotaExhausted(body)
	}
	return status >= 500
}

// isSetupFailure は「設定を直さないと何度やっても同じ」ステータスか。
// 401/403 は認証、404 はモデル名やエンドポイントの誤り。
// これらを破損率に数えると、設定ミスの実行が破損率100%という
// もっともらしい数字を出してしまう。
//
// 429 の残高切れもここに入れる。OpenAI は一時的なレート制限と残高切れを
// 同じ 429 で返すが、本文の code で区別できる（#1634）。
//
//	rate_limit_exceeded  一時的。再試行してよい
//	insufficient_quota   残高切れ。何度やっても同じ
//
// 分けずにネットワーク系へ寄せていたため、残高が尽きた実行が全件失敗のまま
// 完走し「破損率 0.0% / 弁別力 +0.000」というもっともらしい報告を出していた。
func isSetupFailure(status int, body []byte) bool {
	if status == http.StatusTooManyRequests {
		return quotaExhausted(body)
	}
	return status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusNotFound
}

// quotaExhausted は 429 の本文が残高切れを示すか。
//
// 判定は error.code を正とする。type は insufficient_quota でも
// "insufficient_quota" と "billing" の揺れがあるため、code が取れないときの
// 保険としてだけ本文の部分一致を見る。
func quotaExhausted(body []byte) bool {
	var parsed struct {
		Error struct {
			Code string `json:"code"`
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil {
		for _, v := range []string{parsed.Error.Code, parsed.Error.Type} {
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "insufficient_quota", "credit_balance_exhausted":
				return true
			}
		}
		if parsed.Error.Code != "" || parsed.Error.Type != "" {
			return false
		}
	}
	// JSON として読めない・code も type も空のときだけ本文を見る。
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "insufficient_quota") ||
		strings.Contains(lower, "credit_balance_exhausted")
}

// callTimeout は1回の呼び出しの上限。本番のクライアントは60〜90秒で切っている。
const callTimeout = 120 * time.Second

// apiBaseURL は OpenAI 互換エンドポイント。ローカル推論で測るときは env で差し替える。
func apiBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://api.openai.com/v1"
}

// ErrNoAPIKey は OPENAI_API_KEY 未設定。
var ErrNoAPIKey = errors.New("OPENAI_API_KEY が未設定")

// postJSON は JSON を POST して本文を返す。レイテンシは呼び出し側で測る。
func postJSON(ctx context.Context, url string, payload any, header map[string]string) (int, []byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: callTimeout}).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, respBody, nil
}

// CallChatCompletions は /chat/completions を1回だけ叩く。
//
// 本番のクライアント（openai.Client）を通さず生の HTTP で叩く理由は3つある。
//  1. usage（実トークン数）と finish_reason が本番クライアントの戻り値に無い。
//     コストと「出力上限到達」はこの2つが無いと測れない。
//  2. 本番クライアントは最大5回リトライし、温度やモデルまでフォールバックする。
//     それを通すと破損率は「リトライ後の値」になり、プロンプト変更の効果が消える。
//     ここで測るのはモデル素の破損率。ユーザー体験としての失敗率とは別物であり、
//     README にその違いを書いてある。
//  3. 再現性の測定で、クライアント側のキャッシュ・フォールバックの影響を避けたい。
func CallChatCompletions(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64, maxTokens int, jsonMode bool) CallResult {
	key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if key == "" {
		return CallResult{Err: ErrNoAPIKey, Fatal: true}
	}
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature":           temperature,
		"max_completion_tokens": maxTokens,
	}
	if jsonMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}

	start := time.Now()
	status, body, err := postJSON(ctx, apiBaseURL()+"/chat/completions", payload, map[string]string{
		"Authorization": "Bearer " + key,
	})
	latency := time.Since(start).Milliseconds()
	if err != nil {
		// タイムアウト・名前解決失敗などの通信エラー
		return CallResult{LatencyMS: latency, Err: err, Unmeasured: true}
	}
	if status != http.StatusOK {
		return CallResult{LatencyMS: latency, Fatal: isSetupFailure(status, body), Unmeasured: isUnmeasurable(status, body),
			Err: fmt.Errorf("chat/completions HTTP %d: %s", status, snippet(body))}
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return CallResult{LatencyMS: latency, Err: err}
	}
	if len(parsed.Choices) == 0 {
		return CallResult{LatencyMS: latency, Err: errors.New("choices が空")}
	}
	return CallResult{
		Text:             parsed.Choices[0].Message.Content,
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
		LatencyMS:        latency,
		Truncated:        parsed.Choices[0].FinishReason == "length",
	}
}

// CallResponses は /responses を1回だけ叩く（履歴書レビューの本番経路）。
// リトライしない理由は CallChatCompletions と同じ。
//
// jsonMode は text.format.type=json_object を送るかどうか。/responses での
// JSON mode の指定方法で、Chat Completions の response_format と同じ強制になる。
// 本番（openai.Client.ResponsesJSONWithMaxTokens）と同じ値を渡すこと。
// ここだけ外すと指示遵守率が本番と別物になる（#1583）。
func CallResponses(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64, maxOutputTokens int, jsonMode bool) CallResult {
	key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if key == "" {
		return CallResult{Err: ErrNoAPIKey, Fatal: true}
	}
	// 本番（openai.Client.ResponsesJSONWithMaxTokens）と同じ input 形式にする。
	input := []map[string]any{
		{"role": "system", "content": []map[string]string{{"type": "input_text", "text": systemPrompt}}},
		{"role": "user", "content": []map[string]string{{"type": "input_text", "text": userPrompt}}},
	}
	payload := map[string]any{
		"model":             model,
		"input":             input,
		"temperature":       temperature,
		"max_output_tokens": maxOutputTokens,
	}
	if jsonMode {
		payload["text"] = map[string]any{"format": map[string]string{"type": "json_object"}}
	}

	start := time.Now()
	status, body, err := postJSON(ctx, apiBaseURL()+"/responses", payload, map[string]string{
		"Authorization": "Bearer " + key,
	})
	latency := time.Since(start).Milliseconds()
	if err != nil {
		// タイムアウト・名前解決失敗などの通信エラー
		return CallResult{LatencyMS: latency, Err: err, Unmeasured: true}
	}
	if status != http.StatusOK {
		return CallResult{LatencyMS: latency, Fatal: isSetupFailure(status, body), Unmeasured: isUnmeasurable(status, body),
			Err: fmt.Errorf("responses HTTP %d: %s", status, snippet(body))}
	}

	var parsed struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Status            string `json:"status"`
		IncompleteDetails struct {
			Reason string `json:"reason"`
		} `json:"incomplete_details"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return CallResult{LatencyMS: latency, Err: err}
	}

	text := strings.TrimSpace(parsed.OutputText)
	if text == "" {
		var parts []string
		for _, o := range parsed.Output {
			for _, c := range o.Content {
				if strings.TrimSpace(c.Text) != "" {
					parts = append(parts, strings.TrimSpace(c.Text))
				}
			}
		}
		text = strings.Join(parts, "\n")
	}
	return CallResult{
		Text:             text,
		PromptTokens:     parsed.Usage.InputTokens,
		CompletionTokens: parsed.Usage.OutputTokens,
		LatencyMS:        latency,
		Truncated:        parsed.IncompleteDetails.Reason == "max_output_tokens",
	}
}

// snippet はエラー本文を切り詰める。APIキーは本文に含まれないが長さは抑える。
func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		return s[:300]
	}
	return s
}
