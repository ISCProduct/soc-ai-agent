package companyfetch

// 実行: cd Backend && go test ./internal/companyfetch/ -run TestUserAgent -v

import (
	"strings"
	"testing"
)

func TestUserAgent(t *testing.T) {
	tests := []struct {
		name    string
		contact string
		want    string
	}{
		{"連絡先を載せる", "https://bot.example.test/", "Mozilla/5.0 (compatible; SocAI/1.0; +https://bot.example.test/)"},
		{"前後の空白は落とす", "  mailto:bot@example.test ", "Mozilla/5.0 (compatible; SocAI/1.0; +mailto:bot@example.test)"},
		// 仮のURLで他人のドメインを名乗らない
		{"未設定なら連絡先を載せない", "", "Mozilla/5.0 (compatible; SocAI/1.0)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("COMPANY_BOT_CONTACT", tt.contact)
			got := UserAgent()
			if got != tt.want {
				t.Errorf("UserAgent() = %q, want %q", got, tt.want)
			}
			if strings.Contains(got, "example.com") {
				t.Errorf("プレースホルダが残っている: %q", got)
			}
		})
	}
}
