package company

// 公式サイトからの本文取得が既定で無効であることを固定する。
// 実行: cd Backend && go test ./internal/services/company/ -run TestWebsiteExtractEnabled -v

import "testing"

func TestWebsiteExtractEnabled(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want bool
	}{
		// 相手サイトの利用規約は企業ごとに異なり機械的に判断できない。
		// robots.txt も未対応。既定で走らせない。
		{"未設定なら無効", "", false},
		{"0なら無効", "0", false},
		{"trueでは有効にしない(明示の1だけ)", "true", false},
		{"1で有効", "1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("COMPANY_WEBSITE_EXTRACT", tt.env)
			if got := websiteExtractEnabled(); got != tt.want {
				t.Errorf("COMPANY_WEBSITE_EXTRACT=%q → %v, want %v", tt.env, got, tt.want)
			}
		})
	}
}
