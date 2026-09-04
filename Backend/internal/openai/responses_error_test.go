package openai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
