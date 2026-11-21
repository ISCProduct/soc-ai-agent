package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"

	"github.com/labstack/echo/v4"
)

// SOCAIAGENT-694: 企業相関図は未認証で叩けるため、IP単位のレート制限で連打を止める。
func TestEchoCorrelationMapRateLimit(t *testing.T) {
	tests := []struct {
		name       string
		perIP      int
		wantStatus []int
	}{
		{"上限内は通す", 2, []int{http.StatusOK, http.StatusOK}},
		{"IP単位の上限超過で429", 1, []int{http.StatusOK, http.StatusTooManyRequests}},
	}

	orig := middleware.CorrelationMapRateLimiter
	t.Cleanup(func() { middleware.CorrelationMapRateLimiter = orig })

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubLimiter{remaining: tt.perIP}
			middleware.CorrelationMapRateLimiter = stub

			e := echo.New()
			h := echoCorrelationMapRateLimit()(func(c echo.Context) error {
				return c.NoContent(http.StatusOK)
			})
			for i, want := range tt.wantStatus {
				req := httptest.NewRequest(http.MethodGet, "/api/companies/1/correlation-map", nil)
				req.RemoteAddr = "203.0.113.5:1234"
				rec := httptest.NewRecorder()
				got := rec.Code
				if err := h(e.NewContext(req, rec)); err != nil {
					he, ok := err.(*echo.HTTPError)
					if !ok {
						t.Fatalf("%d回目: 予期しないエラー: %v", i+1, err)
					}
					got = he.Code
				} else {
					got = rec.Code
				}
				if got != want {
					t.Errorf("%d回目: ステータス=%d, 期待=%d", i+1, got, want)
				}
			}
			if len(stub.keys) == 0 || stub.keys[0] != "203.0.113.5" {
				t.Errorf("IPをキーにしていない: %v", stub.keys)
			}
		})
	}
}

// 相関図の2ルートにレート制限が付いていることを確認する（付け忘れ防止）。
func TestCompanyRoutes_CorrelationMapIsRateLimited(t *testing.T) {
	orig := middleware.CorrelationMapRateLimiter
	t.Cleanup(func() { middleware.CorrelationMapRateLimiter = orig })
	middleware.CorrelationMapRateLimiter = &stubLimiter{remaining: 0}

	e := echo.New()
	noop := func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	SetupCompanyRoutes(e.Group("/api"), nil, "secret", nil, nil, AIBudgetGuards{Immediate: noop, Deferred: noop})
	for _, path := range []string{"/api/companies/1/correlation-map", "/api/companies/1/correlation-map/compare?with=2"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("%s: ステータス=%d, 期待=429", path, rec.Code)
		}
	}
}
