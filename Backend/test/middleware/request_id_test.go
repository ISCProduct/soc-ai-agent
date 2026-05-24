package middleware_test

// リクエストIDミドルウェアのテスト（Issue #403）
// 実行: cd Backend && go test ./test/middleware/... -run TestRequestID -v

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"
)

// TestRequestIDMiddleware_GeneratesIDWhenAbsent はヘッダーがない場合にIDが生成されることを検証する
func TestRequestIDMiddleware_GeneratesIDWhenAbsent(t *testing.T) {
	var capturedID string
	handler := middleware.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = middleware.GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if capturedID == "" {
		t.Error("リクエストIDがコンテキストに設定されていない")
	}
	if w.Header().Get(middleware.RequestIDHeader) == "" {
		t.Error("レスポンスヘッダーにリクエストIDが設定されていない")
	}
}

// TestRequestIDMiddleware_UsesClientProvidedID はクライアントから送信されたIDが優先されることを検証する
func TestRequestIDMiddleware_UsesClientProvidedID(t *testing.T) {
	const clientID = "test-request-id-abc123"
	var capturedID string
	handler := middleware.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = middleware.GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(middleware.RequestIDHeader, clientID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if capturedID != clientID {
		t.Errorf("got %q, want %q", capturedID, clientID)
	}
	if w.Header().Get(middleware.RequestIDHeader) != clientID {
		t.Errorf("レスポンスヘッダーが %q でない", clientID)
	}
}

// TestRequestIDMiddleware_UniquePerRequest は2リクエストで異なるIDが生成されることを検証する
func TestRequestIDMiddleware_UniquePerRequest(t *testing.T) {
	ids := make([]string, 2)
	handler := middleware.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := range 2 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		ids[i] = w.Header().Get(middleware.RequestIDHeader)
	}

	if ids[0] == ids[1] {
		t.Errorf("2リクエストのIDが同一: %q", ids[0])
	}
}

// TestGetRequestID_EmptyWhenNotSet はコンテキストにIDがない場合は空文字が返ることを検証する
func TestGetRequestID_EmptyWhenNotSet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if id := middleware.GetRequestID(req.Context()); id != "" {
		t.Errorf("got %q, want empty", id)
	}
}
