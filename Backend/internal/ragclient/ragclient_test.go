package ragclient

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"
)

func TestSetAuthHeader(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		wantHeader string
	}{
		{name: "トークン設定時はヘッダーを付与する", token: "secret-token", wantHeader: "secret-token"},
		{name: "空白のみのトークンは付与しない", token: "   ", wantHeader: ""},
		{name: "未設定時は付与しない", token: "", wantHeader: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("RAG_INTERNAL_TOKEN", tt.token)

			req, err := http.NewRequest(http.MethodGet, "http://rag-review:9000/health", nil)
			if err != nil {
				t.Fatalf("リクエスト生成に失敗: %v", err)
			}
			SetAuthHeader(req)

			if got := req.Header.Get(InternalTokenHeader); got != tt.wantHeader {
				t.Errorf("ヘッダー = %q, want %q", got, tt.wantHeader)
			}
		})
	}
}

// SetAuthHeader はリクエストIDをRAGへ伝播する唯一の共通経路なので、
// ヘッダー付与をここで固定する(#1188)。
func TestSetAuthHeaderPropagatesRequestID(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
		wantSet   bool
	}{
		{name: "コンテキストにIDがあれば両ヘッダーを付与", requestID: "abc123", wantSet: true},
		{name: "IDが無ければヘッダーを付与しない", requestID: "", wantSet: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://rag.test/company/context", nil)
			if tt.requestID != "" {
				req = req.WithContext(middleware.WithRequestID(req.Context(), tt.requestID))
			}

			SetAuthHeader(req)

			for _, h := range []string{TraceIDHeader, RequestIDHeader} {
				got := req.Header.Get(h)
				if tt.wantSet && got != tt.requestID {
					t.Errorf("%s = %q, want %q", h, got, tt.requestID)
				}
				if !tt.wantSet && got != "" {
					t.Errorf("%s = %q, want empty", h, got)
				}
			}
		})
	}
}

// ctx を持たない http.NewRequest で作ったリクエストでもパニックしないこと
func TestSetAuthHeaderWithoutContextValue(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://rag.test/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	SetAuthHeader(req)
	if got := req.Header.Get(TraceIDHeader); got != "" {
		t.Errorf("TraceIDHeader = %q, want empty", got)
	}
}
