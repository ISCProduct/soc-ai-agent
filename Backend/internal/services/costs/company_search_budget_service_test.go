package costs

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
)

func TestPostCompanySearchDiscordAlert(t *testing.T) {
	var got atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got.Store(string(body))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := postCompanySearchDiscordAlert(srv.URL, "hello discord"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	raw, _ := got.Load().(string)
	var payload map[string]string
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("invalid json: %v raw=%s", err, raw)
	}
	if payload["content"] != "hello discord" {
		t.Fatalf("content=%q", payload["content"])
	}
}

func TestCompanySearchBudgetService_NotifyDiscord(t *testing.T) {
	var hit atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit.Store(true)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	t.Setenv("COMPANY_SEARCH_ALERT_DISCORD_WEBHOOK_URL", srv.URL)
	t.Setenv("COMPANY_SEARCH_ALERT_SLACK_WEBHOOK_URL", "")
	t.Setenv("REALTIME_ALERT_SLACK_WEBHOOK_URL", "")
	_ = os.Unsetenv("COMPANY_SEARCH_ALERT_EMAILS")
	_ = os.Unsetenv("REALTIME_ALERT_EMAILS")

	s := &CompanySearchBudgetService{
		limit:          10,
		alertThreshold: 5,
		enforce:        true,
	}
	s.notifyIfNeeded(CompanySearchBudgetStatus{
		Month: "2099-01",
		Count: 6,
		Limit: 10,
	})
	if !hit.Load() {
		t.Fatal("expected discord webhook to be called")
	}
}
