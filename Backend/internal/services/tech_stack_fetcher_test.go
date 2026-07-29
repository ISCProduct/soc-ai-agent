package services

import (
	"strings"
	"testing"
)

func TestTechAcquirePrompts_IT(t *testing.T) {
	schema, system, search, label := techAcquirePrompts("IT・ソフトウェア", "サンプル株式会社", "https://example.com")
	if label != "技術スタック" {
		t.Fatalf("label=%q", label)
	}
	if !strings.Contains(schema, "言語・フレームワーク") {
		t.Fatalf("IT schema should mention languages: %s", schema)
	}
	if !strings.Contains(system, "技術スタック") {
		t.Fatalf("IT system prompt unexpected: %s", system)
	}
	if !strings.Contains(search, "日本のIT企業") || !strings.Contains(search, "サンプル株式会社") {
		t.Fatalf("IT search prompt unexpected: %s", search)
	}
}

func TestTechAcquirePrompts_Manufacturing(t *testing.T) {
	schema, system, search, label := techAcquirePrompts("製造業", "ものづくり株式会社", "https://mfg.example.com")
	if label != "設備・技術" {
		t.Fatalf("label=%q", label)
	}
	if !strings.Contains(schema, "生産設備") {
		t.Fatalf("manufacturing schema should mention equipment: %s", schema)
	}
	if !strings.Contains(system, "製造業") {
		t.Fatalf("manufacturing system prompt unexpected: %s", system)
	}
	if !strings.Contains(search, "日本の製造企業") || strings.Contains(search, "日本のIT企業") {
		t.Fatalf("manufacturing search prompt unexpected: %s", search)
	}
}

func TestWebsiteURLOrUnknown(t *testing.T) {
	if got := websiteURLOrUnknown(""); got != "不明" {
		t.Fatalf("empty -> %q", got)
	}
	if got := websiteURLOrUnknown("https://example.com"); got != "https://example.com" {
		t.Fatalf("url -> %q", got)
	}
}
