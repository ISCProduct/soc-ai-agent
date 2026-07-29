package services

import (
	"testing"
	"time"
)

func TestNormalizeCompanyKey_Extended(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"通常", "株式会社テスト", "テスト"},
		{"(株)除去", "(株)サンプル", "サンプル"},
		{"全角(株)除去", "（株）サンプル", "サンプル"},
		{"空白除去", " テ ス ト ", "テスト"},
		{"全角スペース除去", "テスト　カンパニー", "テストカンパニー"},
		{"小文字化", "ABC Corp", "abccorp"},
		{"空文字", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeCompanyKey(tt.in); got != tt.want {
				t.Errorf("normalizeCompanyKey(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNonEmptyURLs(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want int
	}{
		{"空文字", "", 0},
		{"空白のみ", "  ", 0},
		{"有効URL", "https://example.com", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nonEmptyURLs(tt.url)
			if len(got) != tt.want {
				t.Errorf("nonEmptyURLs(%q) len = %d, want %d", tt.url, len(got), tt.want)
			}
		})
	}
}

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"短い文字列", "abc", 5, "abc"},
		{"ちょうど", "abcde", 5, "abcde"},
		{"切り詰め", "abcdef", 5, "abcde…"},
		{"日本語", "あいうえお", 3, "あいう…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateRunes(tt.s, tt.max); got != tt.want {
				t.Errorf("truncateRunes(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}
}

func TestPurgeExpired(t *testing.T) {
	s := &CompanyValidationService{
		cache: map[string]companyValidationCacheEntry{
			"expired": {
				result:    CompanyValidationResult{CanonicalName: "old"},
				expiresAt: time.Now().Add(-1 * time.Hour),
			},
			"valid": {
				result:    CompanyValidationResult{CanonicalName: "new"},
				expiresAt: time.Now().Add(1 * time.Hour),
			},
		},
	}
	s.purgeExpired()

	if _, ok := s.cache["expired"]; ok {
		t.Error("expired entry should have been purged")
	}
	if _, ok := s.cache["valid"]; !ok {
		t.Error("valid entry should not have been purged")
	}
}
