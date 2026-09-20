package routes_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/routes"

	"github.com/labstack/echo/v4"
)

// グローバルのボディ上限を外す対象を固定する。
// 動画以外まで外すと ParseMultipartForm がディスクを埋められる状態に戻る。
func TestBodyLimitSkipper(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/api/interviews/1/upload-video", true},
		{"/api/interviews/999/upload-video", true},
		{"/api/resume/upload", false},
		{"/api/chat", false},
		{"/api/interviews/1/upload-video/extra", false},
	}
	e := echo.New()
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodPost, tt.path, nil)
		c := e.NewContext(req, httptest.NewRecorder())
		if got := routes.BodyLimitSkipper(c); got != tt.want {
			t.Errorf("BodyLimitSkipper(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
