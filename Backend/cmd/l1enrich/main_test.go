package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

func fakeClaude(t *testing.T, text string) anthropic.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"m","type":"message","role":"assistant","model":"x","stop_reason":"end_turn","content":[{"type":"text","text":` + quote(text) + `}],"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	t.Cleanup(srv.Close)
	return anthropic.NewClient(option.WithAPIKey("k"), option.WithBaseURL(srv.URL), option.WithMaxRetries(0))
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestEnrich(t *testing.T) {
	c := fakeClaude(t, "結果です: {\"industry\":\"ソフトウェア\",\"is_it_company\":true,\"website_url\":\"https://example.co.jp\",\"description\":\"業務ソフト\"}")
	r, err := enrich(context.Background(), c, "m", "株式会社テスト")
	if err != nil || r.Industry != "ソフトウェア" || !r.IsIT || r.WebsiteURL != "https://example.co.jp" || r.Name != "株式会社テスト" {
		t.Fatalf("r=%+v err=%v", r, err)
	}
}

func TestEnrichRejectsBadValues(t *testing.T) {
	// 選択肢外の業種と、http でない URL は採用しない。
	c := fakeClaude(t, `{"industry":"宇宙","is_it_company":true,"website_url":"javascript:x","description":""}`)
	r, err := enrich(context.Background(), c, "m", "A")
	if err != nil || r.Industry != "" || r.WebsiteURL != "" {
		t.Fatalf("r=%+v err=%v", r, err)
	}
}

func TestApply(t *testing.T) {
	rows := [][]string{
		{"name", "industry", "segment", "website_url", "publish", "description"},
		{"A", "情報通信業", "core", "", "true", ""},
		{"B", "情報通信業", "sme_si", "", "true", ""},
		{"C", "情報通信業", "sme_si", "", "true", ""},
	}
	cache := map[string]result{
		"A": {IsIT: true, Industry: "ソフトウェア", WebsiteURL: "https://a.example", Description: "業務ソフト"},
		"B": {IsIT: false},
	}
	out, dropped := apply(rows, cache, false)
	if dropped != 1 || len(out) != 3 {
		t.Fatalf("dropped=%d len=%d", dropped, len(out))
	}
	if out[1][1] != "ソフトウェア" || out[1][3] != "https://a.example" || out[1][5] != "業務ソフト" {
		t.Fatalf("A が更新されていない: %v", out[1])
	}
	if out[2][0] != "C" || out[2][1] != "情報通信業" { // 未調査はそのまま残す
		t.Fatalf("C: %v", out[2])
	}
}
