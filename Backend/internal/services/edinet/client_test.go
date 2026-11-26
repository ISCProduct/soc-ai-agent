package edinet

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// 認証エラーは HTTP 200 の JSON で返るので、空の一覧として扱わない（レビュー指摘）。
func TestClient_ErrorBodyWithStatus200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"StatusCode":401,"message":"Access denied due to invalid subscription key."}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "dummy-key")
	if _, err := c.ListDocuments(context.Background(), time.Now()); err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("認証エラーを検出していない: %v", err)
	}
	if _, err := c.FetchCSVZip(context.Background(), "S1"); err == nil {
		t.Error("書類取得でも認証エラーを検出する")
	}
}

// URL の解析エラーにキーを含めない。
func TestClient_RequestErrorDoesNotLeakKey(t *testing.T) {
	c := NewClient("https://api.example.com/%zz", "secret-key-123")
	_, err := c.ListDocuments(context.Background(), time.Now())
	if err == nil || strings.Contains(err.Error(), "secret-key-123") {
		t.Errorf("エラー文にキーが含まれる、またはエラーにならない: %v", err)
	}
}
