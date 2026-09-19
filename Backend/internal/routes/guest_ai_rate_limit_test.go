package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"

	"github.com/labstack/echo/v4"
)

// stubLimiter は指定回数だけ許可するテスト用レート制限器
type stubLimiter struct {
	remaining int
	keys      []string
}

func (s *stubLimiter) Allow(key string) bool {
	s.keys = append(s.keys, key)
	if s.remaining <= 0 {
		return false
	}
	s.remaining--
	return true
}

// TestEchoGuestAIRateLimit は未認証AI経路がIP単位・全体の両方で遮断されることを検証する（#1154）
func TestEchoGuestAIRateLimit(t *testing.T) {
	tests := []struct {
		name       string
		perIP      int
		global     int
		wantStatus []int
	}{
		{"上限内は通す", 3, 3, []int{http.StatusOK, http.StatusOK, http.StatusOK}},
		{"IP単位の上限超過で429", 1, 10, []int{http.StatusOK, http.StatusTooManyRequests}},
		{"全体上限の超過で429（IP分散でも止まる）", 10, 1, []int{http.StatusOK, http.StatusTooManyRequests}},
	}

	origIP, origGlobal := middleware.GuestAIRateLimiter, middleware.GuestAIGlobalRateLimiter
	t.Cleanup(func() {
		middleware.GuestAIRateLimiter, middleware.GuestAIGlobalRateLimiter = origIP, origGlobal
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware.GuestAIRateLimiter = &stubLimiter{remaining: tt.perIP}
			middleware.GuestAIGlobalRateLimiter = &stubLimiter{remaining: tt.global}

			e := echo.New()
			h := echoGuestAIRateLimit()(func(c echo.Context) error {
				return c.NoContent(http.StatusOK)
			})

			for i, want := range tt.wantStatus {
				req := httptest.NewRequest(http.MethodPost, "/api/es/review", nil)
				req.RemoteAddr = "203.0.113.5:1234"
				rec := httptest.NewRecorder()
				c := e.NewContext(req, rec)

				err := h(c)
				got := rec.Code
				if err != nil {
					if he, ok := err.(*echo.HTTPError); ok {
						got = he.Code
					} else {
						t.Fatalf("%d回目: 予期しないエラー: %v", i+1, err)
					}
				}
				if got != want {
					t.Errorf("%d回目: ステータス=%d, 期待=%d", i+1, got, want)
				}
			}
		})
	}
}
