package company

import (
	"reflect"
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
		want []string
	}{
		{"空文字", "", []string{}},
		{"空白のみ", "  ", []string{}},
		{"有効URL", "https://example.com", []string{"https://example.com"}},
		{"前後空白付きURL", "  https://example.com  ", []string{"https://example.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nonEmptyURLs(tt.url)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("nonEmptyURLs(%q) = %v, want %v", tt.url, got, tt.want)
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
	now := time.Now()
	tests := []struct {
		name         string
		cache        map[string]companyValidationCacheEntry
		wantRemaining []string
	}{
		{
			name: "期限切れと有効が混在",
			cache: map[string]companyValidationCacheEntry{
				"expired": {
					result:    CompanyValidationResult{CanonicalName: "old"},
					expiresAt: now.Add(-1 * time.Hour),
				},
				"valid": {
					result:    CompanyValidationResult{CanonicalName: "new"},
					expiresAt: now.Add(1 * time.Hour),
				},
			},
			wantRemaining: []string{"valid"},
		},
		{
			name: "複数期限切れ",
			cache: map[string]companyValidationCacheEntry{
				"expired1": {
					result:    CompanyValidationResult{CanonicalName: "a"},
					expiresAt: now.Add(-2 * time.Hour),
				},
				"expired2": {
					result:    CompanyValidationResult{CanonicalName: "b"},
					expiresAt: now.Add(-30 * time.Minute),
				},
				"valid": {
					result:    CompanyValidationResult{CanonicalName: "c"},
					expiresAt: now.Add(1 * time.Hour),
				},
			},
			wantRemaining: []string{"valid"},
		},
		{
			name: "全て有効",
			cache: map[string]companyValidationCacheEntry{
				"a": {
					result:    CompanyValidationResult{CanonicalName: "a"},
					expiresAt: now.Add(1 * time.Hour),
				},
				"b": {
					result:    CompanyValidationResult{CanonicalName: "b"},
					expiresAt: now.Add(2 * time.Hour),
				},
			},
			wantRemaining: []string{"a", "b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &CompanyValidationService{cache: tt.cache}
			s.purgeExpired()

			if len(s.cache) != len(tt.wantRemaining) {
				t.Fatalf("remaining count = %d, want %d; cache=%v", len(s.cache), len(tt.wantRemaining), s.cache)
			}
			for _, key := range tt.wantRemaining {
				if _, ok := s.cache[key]; !ok {
					t.Errorf("expected key %q to remain", key)
				}
			}
		})
	}
}
