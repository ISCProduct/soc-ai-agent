package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// 実際に OpenAI（および OpenAI 互換実装）が 400 で返す本文。
// isUnsupportedJSONModeErr の判定条件の根拠であり、文面が変わったら
// ここを実測値で差し替える（#1595）。
const (
	// #1595 で実測。JSON mode はプロンプトに "json" の語を要求する。
	errBody400JSONWordMissing = `{"error":{"message":"Response input messages must contain the word 'json' in some form to use 'text.format' of type 'json_object'.","type":"invalid_request_error","param":"text.format","code":null}}`
	// 既存の isUnsupportedTemperatureErr が拾う "Unsupported parameter: 'temperature' ..." と同型。
	errBody400UnsupportedParam = `{"error":{"message":"Unsupported parameter: 'text.format' is not supported with this model.","type":"invalid_request_error","param":"text.format","code":"unsupported_parameter"}}`
	// パラメータ自体を知らないエンドポイント（OpenAI 互換実装で起きる）。
	errBody400UnknownParam = `{"error":{"message":"Unknown parameter: 'text.format'.","type":"invalid_request_error","param":"text.format","code":"unknown_parameter"}}`
	// 値だけ非対応（json_schema しか受けない実装）。
	errBody400InvalidValue = `{"error":{"message":"Invalid value: 'json_object'. Supported values are: 'text' and 'json_schema'.","type":"invalid_request_error","param":"text.format.type","code":"invalid_value"}}`
	// OpenAI 互換サーバの素文（JSON ですらない本文）。
	errBody400PlainText = `response_format json_object is not supported by this model`

	// 以下は JSON mode とは無関係な 400。形式を落としても解決しないので退避してはいけない。
	errBody400PromptTooLong = `{"error":{"message":"Invalid 'input': string too long. Expected a string with maximum length 1048576, but got a string with length 2000000 instead.","type":"invalid_request_error","param":"input","code":"string_above_max_length"}}`
	errBody400ContentFilter = `{"error":{"message":"Invalid prompt: your prompt was flagged as potentially violating our usage policy.","type":"invalid_request_error","param":null,"code":"invalid_prompt"}}`
	errBody400MissingParam  = `{"error":{"message":"Missing required parameter: 'model'.","type":"invalid_request_error","param":"model","code":"missing_required_parameter"}}`
)

// newJSONModeRejectingServer は「text.format 付きのリクエストだけ失敗させる」スタブ。
// 送られた text.format.type を順に記録する（未指定は空文字列）。
func newJSONModeRejectingServer(t *testing.T, status int, errBody, okBody string) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var formats []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Text struct {
				Format struct {
					Type string `json:"type"`
				} `json:"format"`
			} `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("リクエストの解析に失敗: %v", err)
		}
		mu.Lock()
		formats = append(formats, req.Text.Format.Type)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if req.Text.Format.Type != "" {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(errBody))
			return
		}
		_, _ = w.Write([]byte(okBody))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), formats...)
	}
}

// TestResponsesJSONWithMaxTokens_JSONmode非対応の400で退避する は、
// JSON mode を 400 で拒否されたら text.format を落として取得できることを固定する（#1595）。
//
// 退避が無いと外側のリトライが5回とも同じ400を踏み、スリープ分だけ空転してから
// 失敗する。経過時間も見ているのはそこが利用者から見た被害だから。
func TestResponsesJSONWithMaxTokens_JSONmode非対応の400で退避する(t *testing.T) {
	const okBody = `{"output_text":"{\"summary\":\"ok\"}"}`

	tests := map[string]string{
		"プロンプトにjsonの語が無い": errBody400JSONWordMissing,
		"パラメータ非対応":        errBody400UnsupportedParam,
		"パラメータを知らない":      errBody400UnknownParam,
		"値だけ非対応":          errBody400InvalidValue,
		"JSONではない素文":      errBody400PlainText,
	}

	for name, errBody := range tests {
		t.Run(name, func(t *testing.T) {
			srv, formats := newJSONModeRejectingServer(t, http.StatusBadRequest, errBody, okBody)
			cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")

			start := time.Now()
			out, err := cli.ResponsesJSONWithMaxTokens(context.Background(), "systemはJSONを求める", "user", 0.2, 500, "gpt-4o-mini")
			elapsed := time.Since(start)
			if err != nil {
				t.Fatalf("退避して成功するべき: %v", err)
			}
			if out != `{"summary":"ok"}` {
				t.Errorf("本文 = %q", out)
			}
			// 1回目は JSON mode、2回目は text.format なし。この順序が退避そのもの。
			got := formats()
			want := []string{"json_object", ""}
			if len(got) != len(want) {
				t.Fatalf("送った text.format.type = %v, want %v", got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("%d回目の text.format.type = %q, want %q", i+1, got[i], want[i])
				}
			}
			// バックオフのスリープ（最短でも2秒）に入っていないこと。
			if elapsed > 2*time.Second {
				t.Errorf("退避せずに空転している（経過 %v）", elapsed)
			}
		})
	}
}

// TestResponsesJSONWithMaxTokens_JSONmode以外の400では退避しない は、
// 形式を落としても解決しない 400 で JSON mode を外さないことを固定する（#1595）。
//
// 退避条件を広げすぎると、プロンプト長超過やコンテンツフィルタの 400 で
// 形式だけ落ちた要求を投げ直すことになり、JSON mode が黙って無効化される。
//
// これらは従来どおり外側のリトライへ渡るため、待ち時間の短いコンテキストで打ち切る。
func TestResponsesJSONWithMaxTokens_JSONmode以外の400では退避しない(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "プロンプト長超過", status: http.StatusBadRequest, body: errBody400PromptTooLong},
		{name: "コンテンツフィルタ", status: http.StatusBadRequest, body: errBody400ContentFilter},
		{name: "必須パラメータ不足", status: http.StatusBadRequest, body: errBody400MissingParam},
		// 文面は JSON mode 非対応そのものだが 400 ではない。5xx は再試行すれば
		// 成功しうるので、形式を落として品質を下げる前に投げ直す。
		{name: "同じ文面でも5xxなら退避しない", status: http.StatusBadGateway, body: errBody400UnsupportedParam},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, formats := newJSONModeRejectingServer(t, tt.status, tt.body, `{"output_text":"{}"}`)
			cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")

			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			if _, err := cli.ResponsesJSONWithMaxTokens(ctx, "systemはJSONを求める", "user", 0.2, 500, "gpt-4o-mini"); err == nil {
				t.Fatal("エラーになるべき")
			}
			got := formats()
			if len(got) == 0 {
				t.Fatal("リクエストが送られていない")
			}
			for i, format := range got {
				if format != "json_object" {
					t.Errorf("%d回目で JSON mode が外れた（text.format.type = %q）", i+1, format)
				}
			}
		})
	}
}

// TestResponsesJSONWithMaxTokens_退避後の出力はそのまま返す は、
// 退避後にモデルが返しがちなコードフェンス付きの本文を、クライアントが
// 加工せず呼び出し側へ渡すことを固定する（#1595）。
//
// 切り出しは呼び出し側の decodeJSON（復旧処理）の担当で、
// クライアント側で二重に剥がすと復旧処理の入力が変わってしまう。
func TestResponsesJSONWithMaxTokens_退避後の出力はそのまま返す(t *testing.T) {
	const fenced = "```json\\n{\\\"summary\\\":\\\"ok\\\"}\\n```"
	srv, _ := newJSONModeRejectingServer(t, http.StatusBadRequest, errBody400JSONWordMissing, `{"output_text":"`+fenced+`"}`)
	cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")

	out, err := cli.ResponsesJSONWithMaxTokens(context.Background(), "systemはJSONを求める", "user", 0.2, 500, "gpt-4o-mini")
	if err != nil {
		t.Fatalf("退避して成功するべき: %v", err)
	}
	if !strings.HasPrefix(out, "```json") || !strings.Contains(out, `{"summary":"ok"}`) {
		t.Errorf("本文が加工されている: %q", out)
	}
}
