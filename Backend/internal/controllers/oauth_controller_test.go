package controllers

import "testing"

func TestBuildTenantURL(t *testing.T) {
	cases := []struct {
		name   string
		appURL string
		slug   string
		want   string
	}{
		{"通常のslug", "https://shukatsu-ai.jp", "acme", "https://acme.shukatsu-ai.jp"},
		{"slugが空はappURLのまま", "https://shukatsu-ai.jp", "", "https://shukatsu-ai.jp"},
		{"ローカル開発URLでも動く", "http://localhost:3000", "acme", "http://acme.localhost:3000"},
		{"ホストが無いappURLはそのまま返す", "not-a-valid-host", "acme", "not-a-valid-host"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTenantURL(tt.appURL, tt.slug)
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}
