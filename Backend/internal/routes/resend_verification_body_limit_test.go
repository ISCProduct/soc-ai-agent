package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/middleware"

	"github.com/labstack/echo/v4"
)

// 認証メール再送は認証不要の入口なので、全体の上限(32MB)より小さい上限(4KB)を、この経路にだけ置く。
// 巨大なボディが、読み込み(Bind)や制限器のキーに届く前に、413 で止まることを確かめる。

type allowAllLimiter struct{}

func (allowAllLimiter) Allow(string) bool { return true }

func TestResendVerificationRoute_RejectsLargeBodyBeforeHandler(t *testing.T) {
	orig := middleware.VerificationResendRateLimiter
	middleware.VerificationResendRateLimiter = allowAllLimiter{}
	t.Cleanup(func() { middleware.VerificationResendRateLimiter = orig })

	e := echo.New()
	// コントローラは nil のまま。ハンドラーまで届けば nil 参照で落ちる(echo の Recover が無いので panic になる)。
	// 413 で止まっていること自体が、ボディ上限が先に動いている証拠になる。
	SetupAuthRoutes(e.Group("/api"), nil, nil, "secret", nil, nil)

	big := `{"email":"` + strings.Repeat("a", 5000) + `@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-verification", strings.NewReader(big))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}
