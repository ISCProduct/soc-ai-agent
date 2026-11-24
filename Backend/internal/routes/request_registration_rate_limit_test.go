package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/middleware"

	"github.com/labstack/echo/v4"
)

// 登録申請のルートは、IP 単位の上限を超えると、ハンドラー(メール送信)に届く前に 429 で止まる。
func TestRequestRegistrationRoute_IsRateLimitedByIP(t *testing.T) {
	orig := middleware.RegistrationRequestRateLimiter
	limiter := &denyAllLimiter{}
	middleware.RegistrationRequestRateLimiter = limiter
	t.Cleanup(func() { middleware.RegistrationRequestRateLimiter = orig })

	e := echo.New()
	// コントローラは nil。ハンドラーまで届けば nil 参照で落ちるので、429 はミドルウェアが先に動いた証拠。
	SetupAuthRoutes(e.Group("/api"), nil, nil, "secret", nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/request-registration", strings.NewReader(`{"email":"user@example.com"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = "203.0.113.9:4321"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if len(limiter.keys) != 1 || limiter.keys[0] != "203.0.113.9" {
		t.Fatalf("IP をキーに制限する: %v", limiter.keys)
	}
}
