package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"

	"github.com/labstack/echo/v4"
)

// ゲスト作成は IP 単位の上限を超えると、ハンドラー（ユーザー作成）に届く前に 429 で止まる (SOCAIAGENT-689)。
func TestGuestRoute_IsRateLimitedByIP(t *testing.T) {
	orig := middleware.GuestCreateRateLimiter
	limiter := &denyAllLimiter{}
	middleware.GuestCreateRateLimiter = limiter
	t.Cleanup(func() { middleware.GuestCreateRateLimiter = orig })

	e := echo.New()
	// コントローラは nil。ハンドラーまで届けば nil 参照で落ちるので、429 自体がミドルウェアが先に動いた証拠になる。
	SetupAuthRoutes(e.Group("/api"), nil, nil, "secret", nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/guest", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if len(limiter.keys) != 1 || limiter.keys[0] != "203.0.113.10" {
		t.Fatalf("IP をキーに制限する: %v", limiter.keys)
	}
}
