package main

import "testing"

func TestIsAllowedOrigin(t *testing.T) {
	exact := map[string]struct{}{"https://shukatsu-ai.jp": {}}
	wildcards := []wildcardPattern{{prefix: "https://", suffix: ".shukatsu-ai.jp"}}

	cases := []struct {
		name   string
		origin string
		want   bool
	}{
		{"exact match", "https://shukatsu-ai.jp", true},
		{"wildcard subdomain", "https://my-school.shukatsu-ai.jp", true},
		{"wildcard admin subdomain", "https://admin.shukatsu-ai.jp", true},
		{"different scheme not matched by wildcard", "http://my-school.shukatsu-ai.jp", false},
		{"unrelated domain", "https://evil.com", false},
		{"domain that merely ends with the suffix is not treated as evasion since prefix is also required", "https://evilshukatsu-ai.jp", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAllowedOrigin(tc.origin, exact, wildcards); got != tc.want {
				t.Errorf("isAllowedOrigin(%q) = %v, want %v", tc.origin, got, tc.want)
			}
		})
	}
}
