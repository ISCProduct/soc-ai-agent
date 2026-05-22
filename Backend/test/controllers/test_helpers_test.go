package controllers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"

	"github.com/labstack/echo/v4"
)

var testEcho = echo.New()

// withUserID はリクエストコンテキストにuserIDを設定するヘルパー
func withUserID(r *http.Request, userID uint) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey, userID)
	return r.WithContext(ctx)
}

// newCtx はリクエストとレコーダーからecho.Contextを生成する
func newCtx(req *http.Request, rec *httptest.ResponseRecorder) echo.Context {
	return testEcho.NewContext(req, rec)
}

// assertStatus はハンドラーを呼び出してHTTPステータスを検証するヘルパー
func assertStatus(t *testing.T, handler func(echo.Context) error, c echo.Context, expected int) {
	t.Helper()
	err := handler(c)
	if err != nil {
		testEcho.HTTPErrorHandler(err, c)
	}
	rec := c.Response().Writer.(*httptest.ResponseRecorder)
	if rec.Code != expected {
		t.Errorf("expected status %d, got %d", expected, rec.Code)
	}
}
