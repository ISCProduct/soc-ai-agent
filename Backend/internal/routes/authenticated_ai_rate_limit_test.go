package routes

import (
	"Backend/internal/middleware"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestAuthenticatedAIRateLimitSeparatesUsers(t *testing.T) {
	orig := middleware.AuthenticatedAIRateLimiter
	lim := &stubLimiter{remaining: 2}
	middleware.AuthenticatedAIRateLimiter = lim
	t.Cleanup(func() { middleware.AuthenticatedAIRateLimiter = orig })

	e := echo.New()
	h := echoAuthenticatedAIRateLimit()(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})
	for _, id := range []uint{7, 8} {
		req := httptest.NewRequest(http.MethodPost, "/api/chat", nil)
		ctx := context.WithValue(req.Context(), middleware.UserIDContextKey, id)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		if err := h(e.NewContext(req, rec)); err != nil {
			t.Fatalf("user %d: %v", id, err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("user %d status = %d", id, rec.Code)
		}
	}
	if len(lim.keys) != 2 || lim.keys[0] != "user:7" || lim.keys[1] != "user:8" {
		t.Fatalf("keys = %#v", lim.keys)
	}
}

func TestAIRoutesRejectWhenUserRateExceeded(t *testing.T) {
	orig := middleware.AuthenticatedAIRateLimiter
	middleware.AuthenticatedAIRateLimiter = &stubLimiter{remaining: 0}
	t.Cleanup(func() { middleware.AuthenticatedAIRateLimiter = orig })

	const secret = "test-secret"
	noop := func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	budget := AIBudgetGuards{Immediate: noop, Deferred: noop}
	e := echo.New()
	api := e.Group("/api")
	SetupChatRoutes(api, nil, nil, secret, nil, nil, budget)
	SetupInterviewRoutes(api, nil, nil, secret, nil, nil, budget)
	SetupResumeRoutes(api, nil, secret, nil, nil, budget)

	token, err := middleware.GenerateJWT(7, "u@example.com", secret)
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}
	paths := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/chat"},
		{http.MethodPost, "/api/questions/generate"},
		{http.MethodPost, "/api/interviews/1/turn"},
		{http.MethodPost, "/api/interviews/1/start-turn"},
		{http.MethodPost, "/api/interviews/1/report/regenerate"},
		{http.MethodPost, "/api/resume/review"},
		{http.MethodPost, "/api/realtime/token"},
	}
	for _, tt := range paths {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		req.Header.Set("X-User-Token", token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("%s %s status = %d, 期待 429", tt.method, tt.path, rec.Code)
		}
	}
}
