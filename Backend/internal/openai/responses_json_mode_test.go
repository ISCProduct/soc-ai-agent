package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// /responses の JSON mode は text.format.type で指定する（#1583）。
// この値は API 側の仕様なので、書き換えるとモデルが素の JSON を返さなくなる。
// テストで固定しておかないと、指示遵守率 0% の状態へ黙って戻れてしまう。
const wantJSONModeType = "json_object"

// capturedResponsesRequest は /responses へ送られたペイロードのうち検証したい部分。
type capturedResponsesRequest struct {
	Text struct {
		Format struct {
			Type string `json:"type"`
		} `json:"format"`
	} `json:"text"`
	// TextPresent は text キー自体が送られたか。JSON mode 以外では送らない挙動を
	// 「type が空」と区別するために持つ。
	TextPresent bool
}

// newCapturingResponsesServer は /responses のリクエストを記録して固定応答を返す。
func newCapturingResponsesServer(t *testing.T, captured *[]capturedResponsesRequest) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Errorf("リクエストの解析に失敗: %v", err)
		}
		var c capturedResponsesRequest
		if body, ok := raw["text"]; ok {
			c.TextPresent = true
			if err := json.Unmarshal(body, &c.Text); err != nil {
				t.Errorf("text の解析に失敗: %v", err)
			}
		}
		*captured = append(*captured, c)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"{\"summary\":\"ok\"}"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestResponsesWithMaxTokens_JSONmodeの指定 は JSON mode がオプトインであることと、
// 指定したときに送る値を固定する。
func TestResponsesWithMaxTokens_JSONmodeの指定(t *testing.T) {
	tests := []struct {
		name            string
		call            func(cli *Client, ctx context.Context) (string, error)
		wantTextPresent bool
		wantFormatType  string
	}{
		{
			name: "ResponsesWithMaxTokens は出力形式を強制しない",
			call: func(cli *Client, ctx context.Context) (string, error) {
				return cli.ResponsesWithMaxTokens(ctx, "system", "user", 0.2, 500, "gpt-4o-mini")
			},
			wantTextPresent: false,
		},
		{
			name: "ResponsesJSONWithMaxTokens は json_object を送る",
			call: func(cli *Client, ctx context.Context) (string, error) {
				return cli.ResponsesJSONWithMaxTokens(ctx, "systemはJSONを求める", "user", 0.2, 500, "gpt-4o-mini")
			},
			wantTextPresent: true,
			wantFormatType:  wantJSONModeType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured []capturedResponsesRequest
			srv := newCapturingResponsesServer(t, &captured)
			cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")

			out, err := tt.call(cli, context.Background())
			if err != nil {
				t.Fatalf("呼び出しに失敗: %v", err)
			}
			if out == "" {
				t.Fatal("本文が空")
			}
			if len(captured) != 1 {
				t.Fatalf("リクエスト回数 = %d, want 1", len(captured))
			}
			got := captured[0]
			if got.TextPresent != tt.wantTextPresent {
				t.Fatalf("text キーの有無 = %v, want %v", got.TextPresent, tt.wantTextPresent)
			}
			if got.Text.Format.Type != tt.wantFormatType {
				t.Errorf("text.format.type = %q, want %q", got.Text.Format.Type, tt.wantFormatType)
			}
		})
	}
}

// TestResponsesJSONWithMaxTokens_上限到達のやり直しもJSONmodeを保つ は、
// 枠を倍にする再試行で JSON mode が外れないことを固定する。
// ここが外れると2回目だけ素の JSON でなくなり、原因が分かりにくい形で再発する。
func TestResponsesJSONWithMaxTokens_上限到達のやり直しもJSONmodeを保つ(t *testing.T) {
	var captured []capturedResponsesRequest
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Errorf("リクエストの解析に失敗: %v", err)
		}
		var c capturedResponsesRequest
		if body, ok := raw["text"]; ok {
			c.TextPresent = true
			_ = json.Unmarshal(body, &c.Text)
		}
		captured = append(captured, c)
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			// 本文なし＋上限到達。公開メソッドは枠を倍にして呼び直す。
			_, _ = w.Write([]byte(`{"output_text":"","incomplete_details":{"reason":"max_output_tokens"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"output_text":"{\"summary\":\"ok\"}"}`))
	}))
	t.Cleanup(srv.Close)

	cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")
	if _, err := cli.ResponsesJSONWithMaxTokens(context.Background(), "systemはJSONを求める", "user", 0.2, 500, "gpt-4o-mini"); err != nil {
		t.Fatalf("呼び出しに失敗: %v", err)
	}
	if len(captured) < 2 {
		t.Fatalf("やり直しが起きていない（リクエスト回数 = %d）", len(captured))
	}
	for i, c := range captured {
		if !c.TextPresent || c.Text.Format.Type != wantJSONModeType {
			t.Errorf("%d回目の text.format.type = %q (present=%v), want %q", i+1, c.Text.Format.Type, c.TextPresent, wantJSONModeType)
		}
	}
}
