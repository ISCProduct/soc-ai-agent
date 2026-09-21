package scraper_test

import (
	"testing"
	"time"

	"Backend/internal/scraper"

	"github.com/stretchr/testify/assert"
)

// ── ResolveYear() ─────────────────────────────────────────────────────────────

func TestResolveYear_Override(t *testing.T) {
	assert.Equal(t, 2030, scraper.ResolveYear(2030))
	assert.Equal(t, 2025, scraper.ResolveYear(2025))
}

func TestResolveYear_Auto(t *testing.T) {
	year := scraper.ResolveYear(0)
	now := time.Now()
	if now.Month() >= time.April {
		assert.Equal(t, now.Year()+2, year)
	} else {
		assert.Equal(t, now.Year()+1, year)
	}
}

// ── NormalizeName() ───────────────────────────────────────────────────────────

func TestNormalizeName_Abbreviations(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"（株）テスト", "株式会社テスト"},
		{"(株)テスト", "株式会社テスト"},
		{"㈱テスト", "株式会社テスト"},
		{"テスト（有）", "テスト有限会社"},
		{"テスト(有)", "テスト有限会社"},
		{"テスト合同会社", "テスト合同会社"}, // 変換不要
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, scraper.NormalizeName(tt.input))
		})
	}
}

func TestNormalizeName_Whitespace(t *testing.T) {
	assert.Equal(t, "テスト株式会社", scraper.NormalizeName("テスト  株式会社"))
	assert.Equal(t, "テスト株式会社", scraper.NormalizeName("  テスト株式会社  "))
}

// ── Similarity() ──────────────────────────────────────────────────────────────

func TestSimilarity_ExactMatch(t *testing.T) {
	assert.Equal(t, 1.0, scraper.Similarity("株式会社テスト", "株式会社テスト"))
}

func TestSimilarity_NoMatch(t *testing.T) {
	score := scraper.Similarity("株式会社テスト", "全然違う名前XXXX")
	assert.Less(t, score, 0.5)
}

func TestSimilarity_PartialMatch(t *testing.T) {
	score := scraper.Similarity("株式会社テスト", "株式会社テストグループ")
	assert.Greater(t, score, 0.7)
	assert.Less(t, score, 1.0)
}

func TestSimilarity_EmptyStrings(t *testing.T) {
	assert.Equal(t, 1.0, scraper.Similarity("", ""))
	assert.Equal(t, 0.0, scraper.Similarity("テスト", ""))
	assert.Equal(t, 0.0, scraper.Similarity("", "テスト"))
}
