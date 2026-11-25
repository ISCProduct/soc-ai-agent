package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// 空応答が続いても web_search は2回（再試行1回）で諦める（SOCAIAGENT-723）。
// 空応答でも検索料はかかるので、試行回数がそのまま費用になる。
func TestWebSearchJSON_RetriesAtMostOnce(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[]}`))
	}))
	defer srv.Close()

	cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")
	if _, err := cli.WebSearchJSON(context.Background(), "テスト株式会社の事業内容", 600); err == nil {
		t.Fatal("空応答なのにエラーにならない")
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("web_search の呼び出し回数 = %d, want 2", got)
	}
}
