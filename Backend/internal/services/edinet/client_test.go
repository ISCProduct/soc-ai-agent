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

func TestDocument_ViewerURL(t *testing.T) {
	tests := []struct {
		name string
		doc  Document
		want string
	}{
		{"PDF があるときは PDF へ直接つなぐ", Document{DocID: "S100WOE6", PDFFlag: "1"}, "https://disclosure2dl.edinet-fsa.go.jp/searchdocument/pdf/S100WOE6.pdf"},
		{"PDF がないときは閲覧ページ", Document{DocID: "S100WOE6", PDFFlag: "0"}, "https://disclosure2.edinet-fsa.go.jp/WZEK0040.aspx?S100WOE6"},
		{"pdfFlag が無い応答でも閲覧ページ", Document{DocID: "S100WOE6"}, "https://disclosure2.edinet-fsa.go.jp/WZEK0040.aspx?S100WOE6"},
		{"書類 ID の区切り文字はパスに入れない", Document{DocID: "S100/../x", PDFFlag: "1"}, "https://disclosure2dl.edinet-fsa.go.jp/searchdocument/pdf/S100%2F..%2Fx.pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.doc.ViewerURL(); got != tt.want {
				t.Errorf("ViewerURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
