package middleware_test

// リクエストロガーミドルウェアのテスト（Issue #403）
// 実行: cd Backend && go test ./test/middleware/... -run TestRequestLogger -v

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/middleware"
)

// TestRequestLoggerMiddleware_LogsRequest はリクエスト情報がログに記録されることを検証する
func TestRequestLoggerMiddleware_LogsRequest(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	handler := middleware.RequestLoggerMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	logged := buf.String()
	for _, want := range []string{"GET", "/api/health", "200"} {
		if !strings.Contains(logged, want) {
			t.Errorf("ログに %q が含まれていない: %s", want, logged)
		}
	}
}

// TestRequestLoggerMiddleware_IncludesRequestID はX-Request-IDがある場合にログに含まれることを検証する
func TestRequestLoggerMiddleware_IncludesRequestID(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	const testID = "trace-xyz-999"
	handler := middleware.RequestIDMiddleware(
		middleware.RequestLoggerMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/chat", nil)
	req.Header.Set(middleware.RequestIDHeader, testID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !strings.Contains(buf.String(), testID) {
		t.Errorf("ログにrequest_id %q が含まれていない: %s", testID, buf.String())
	}
}

// TestRequestLoggerMiddleware_WarnOnClientError は4xxステータスでWARNレベルで記録されることを検証する
func TestRequestLoggerMiddleware_WarnOnClientError(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	handler := middleware.RequestLoggerMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodGet, "/not-found", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !strings.Contains(buf.String(), "WARN") {
		t.Errorf("4xx は WARN レベルで記録されるべき: %s", buf.String())
	}
}

// TestRequestLoggerMiddleware_ErrorOnServerError は5xxステータスでERRORレベルで記録されることを検証する
func TestRequestLoggerMiddleware_ErrorOnServerError(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	handler := middleware.RequestLoggerMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodGet, "/crash", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !strings.Contains(buf.String(), "ERROR") {
		t.Errorf("5xx は ERROR レベルで記録されるべき: %s", buf.String())
	}
}
