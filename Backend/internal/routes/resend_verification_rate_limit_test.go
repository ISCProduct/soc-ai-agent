package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/middleware"

	"github.com/labstack/echo/v4"
)

// 認証メール再送のルートが登録されていて、IP 単位の上限を超えると、
// ハンドラー(メール送信)に届く前に 429 で止まることを確かめる。

type denyAllLimiter struct{ keys []string }

func (l *denyAllLimiter) Allow(key string) bool { l.keys = append(l.keys, key); return false }

func TestResendVerificationRoute_IsRateLimitedByIP(t *testing.T) {
	orig := middleware.VerificationResendRateLimiter
	limiter := &denyAllLimiter{}
	middleware.VerificationResendRateLimiter = limiter
	t.Cleanup(func() { middleware.VerificationResendRateLimiter = orig })

	e := echo.New()
	// コントローラは nil のまま。ハンドラーまで届けば nil 参照で落ちるので、
	// 429 で止まっていること自体が、ミドルウェアが先に動いている証拠になる。
	SetupAuthRoutes(e.Group("/api"), nil, nil, "secret", nil, nil)

	var registered bool
	for _, r := range e.Routes() {
		if r.Method == http.MethodPost && r.Path == "/api/auth/resend-verification" {
			registered = true
		}
	}
	if !registered {
		t.Fatal("POST /api/auth/resend-verification が登録されていない")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-verification", strings.NewReader(`{"email":"user@example.com"}`))
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
