package companyfetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveSearchProviderName(t *testing.T) {
	t.Setenv("COMPANY_SEARCH_PROVIDER", "")
	t.Setenv("BRAVE_SEARCH_API_KEY", "")
	if got := ResolveSearchProviderName(); got != "openai" {
		t.Fatalf("default=%q", got)
	}
	t.Setenv("COMPANY_SEARCH_PROVIDER", "brave")
	if got := ResolveSearchProviderName(); !strings.Contains(got, "fallback") {
		t.Fatalf("brave without key=%q", got)
	}
	t.Setenv("BRAVE_SEARCH_API_KEY", "test-key")
	if got := ResolveSearchProviderName(); got != "brave" {
		t.Fatalf("brave with key=%q", got)
	}
}

func TestBraveSearchProvider_Search(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Subscription-Token") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"web":{"results":[{"title":"Acme","url":"https://acme.example","description":"IT企業"}]}}`))
	}))
	defer srv.Close()

	p := &BraveSearchProvider{
		APIKey:     "test-key",
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	}
	text, id, err := p.Search(context.Background(), "Acme 株式会社", 500)
	if err != nil {
		t.Fatal(err)
	}
	if id != "brave-search" {
		t.Fatalf("id=%q", id)
	}
	if !strings.Contains(text, "Acme") || !strings.Contains(text, "https://acme.example") {
		t.Fatalf("text=%q", text)
	}
}

func TestNewSearchProviderFromEnv_OpenAIDefault(t *testing.T) {
	t.Setenv("COMPANY_SEARCH_PROVIDER", "openai")
	p := NewSearchProviderFromEnv(nil)
	if p.Name() != "openai" {
		t.Fatalf("name=%q", p.Name())
	}
}
