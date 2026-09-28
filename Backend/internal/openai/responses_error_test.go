package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDoResponses_OutputTruncated は max_output_tokens で切れた応答を検知できることを検証する（#1529）。
//
// 要点は「本文が途中まで返っている」ケースである。旧実装は本文が空のときだけ
// incomplete_details を見ていたため、途中で切れた JSON が正常な応答として
// 呼び出し側へ渡っていた（#1521 と同型）。
func TestDoResponses_OutputTruncated(t *testing.T) {
	// 途中で切れた JSON。decodeJSON 相当の波括弧探索では「読めてしまう」形。
	const partial = `{"scores":{"specificity":4},"items":[{"quote":"あ`
	// output_text に入れるため JSON 文字列としてエスケープする
	escaped, err := json.Marshal(partial)
	if err != nil {
		t.Fatalf("テスト用ボディの組み立てに失敗: %v", err)
	}
	truncatedBody := `{"output_text":` + string(escaped) + `,"incomplete_details":{"reason":"max_output_tokens"}}`

	tests := []struct {
		name          string
		body          string
		withFlag      bool
		wantTruncated bool
		wantErr       bool
	}{
		{
			name:          "上限到達で本文が途中まで",
			body:          truncatedBody,
			withFlag:      true,
			wantTruncated: true,
		},
		{
			name:          "正常な応答ではフラグが立たない",
			body:          `{"output_text":"{\"scores\":{}}"}`,
			withFlag:      true,
			wantTruncated: false,
		},
		{
			name:          "上限到達で本文が空（従来どおりエラー）",
			body:          `{"output":[],"incomplete_details":{"reason":"max_output_tokens"}}`,
			withFlag:      true,
			wantTruncated: true,
			wantErr:       true,
		},
		{
			name:          "フラグを載せない呼び出し側は従来どおり本文を受け取る",
			body:          truncatedBody,
			withFlag:      false,
			wantTruncated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			ctx := context.Background()
			if tt.withFlag {
				ctx = WithTruncationFlag(ctx)
			}
			cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")
			content, err := cli.doResponses(ctx, responsesRequest{Model: "gpt-4o-mini", Input: "q"})
			if tt.wantErr != (err != nil) {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if got := OutputTruncated(ctx); got != tt.wantTruncated {
				t.Errorf("OutputTruncated = %v, want %v（本文=%q）", got, tt.wantTruncated, content)
			}
		})
	}
}

// doResponses が 2xx 以外でステータスコードを保持することを検証する。
// 本文が空でもエラーメッセージが空にならないことが要点（旧実装は errors.New("") を返していた）。
func TestDoResponses_NonOKKeepsStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		retryable  bool
		wantInText string
	}{
		{name: "本文なし404", status: http.StatusNotFound, body: "", retryable: false, wantInText: "404"},
		{name: "本文なし429", status: http.StatusTooManyRequests, body: "", retryable: true, wantInText: "429"},
		{name: "本文あり503", status: http.StatusServiceUnavailable, body: "upstream down", retryable: true, wantInText: "upstream down"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")
			_, err := cli.doResponses(context.Background(), responsesRequest{Model: "gpt-4o-mini", Input: "q"})
			if err == nil {
				t.Fatal("エラーが返るべき")
			}
			if strings.TrimSpace(err.Error()) == "" {
				t.Fatal("エラーメッセージが空になっている")
			}
			if !strings.Contains(err.Error(), tt.wantInText) {
				t.Fatalf("メッセージに %q が含まれない: %s", tt.wantInText, err.Error())
			}
			if got := isRetryableAPIErr(err); got != tt.retryable {
				t.Fatalf("再試行判定 got=%v want=%v", got, tt.retryable)
			}
			// ラップされていても判定できること
			if got := isRetryableAPIErr(fmt.Errorf("web search failed: %w", err)); got != tt.retryable {
				t.Fatalf("ラップ後の再試行判定 got=%v want=%v", got, tt.retryable)
			}
		})
	}
}
