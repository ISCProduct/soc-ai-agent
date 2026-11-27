package company

// 公式サイトからの本文取得が既定で無効であることを固定する。
// 有効化には COMPANY_WEBSITE_EXTRACT=1 に加えて、User-Agent に載せる連絡先
// (COMPANY_BOT_CONTACT) が要る。連絡先の無いボットで相手サイトを読まない。
// 実行: cd Backend && go test ./internal/services/company/ -run TestWebsiteExtractEnabled -v

import "testing"

func TestWebsiteExtractEnabled(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		contact string
		want    bool
	}{
		// 相手サイトの利用規約は企業ごとに異なり機械的に判断できない。既定で走らせない。
		{"未設定なら無効", "", "https://bot.example.test/", false},
		{"0なら無効", "0", "https://bot.example.test/", false},
		{"trueでは有効にしない(明示の1だけ)", "true", "https://bot.example.test/", false},
		{"連絡先が無ければ1でも無効", "1", "", false},
		{"連絡先が空白だけなら無効", "1", "   ", false},
		{"1と連絡先で有効", "1", "https://bot.example.test/", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("COMPANY_WEBSITE_EXTRACT", tt.env)
			t.Setenv("COMPANY_BOT_CONTACT", tt.contact)
			if got := websiteExtractEnabled(); got != tt.want {
				t.Errorf("COMPANY_WEBSITE_EXTRACT=%q COMPANY_BOT_CONTACT=%q → %v, want %v", tt.env, tt.contact, got, tt.want)
			}
		})
	}
}
