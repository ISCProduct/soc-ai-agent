package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestRequiresAIBudget(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   bool
	}{
		{http.MethodPost, "/api/chat", true},
		{http.MethodGet, "/api/chat/analysis", true},
		{http.MethodPost, "/api/chat/send-report", true},
		{http.MethodPost, "/api/interviews", false},
		{http.MethodGet, "/api/interviews/:id/phrase-suggestions", true},
		{http.MethodPost, "/api/interviews/:id/send-report", true},
		{http.MethodPost, "/api/interviews/:id/start", true},
		{http.MethodPost, "/api/realtime/token", true},
		{http.MethodPost, "/api/interviews/:id/utterances", false},
		{http.MethodPost, "/api/interviews/:id/finish", false},
		{http.MethodPost, "/api/interviews/:id/report/regenerate", true},
		{http.MethodGet, "/api/chat", false},
		{http.MethodPost, "/api/es/review", true},
		{http.MethodPost, "/api/github/repo/summarize", true},
		{http.MethodGet, "/api/companies/web-search", true},
		{http.MethodPost, "/api/companies/validate", true},
		{http.MethodGet, "/api/companies/relations", false},
	}

	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			if got := requiresAIBudget(test.method, test.path); got != test.want {
				t.Errorf("requiresAIBudget(%q, %q) = %t, want %t", test.method, test.path, got, test.want)
			}
		})
	}
}

func TestEchoAIBudgetFailsClosedWhenServiceIsMissing(t *testing.T) {
	e := echo.New()
	e.POST("/api/chat", func(echo.Context) error {
		t.Fatal("budget-protected handler should not be reached")
		return nil
	}, EchoAIBudget(nil, ""))

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/chat", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
