package config

import "testing"

func TestIsProduction(t *testing.T) {
	tests := []struct {
		env  string
		want bool
	}{
		{"production", true},
		{"staging", false},
		{"", false},
		{" production", false}, // 元の実装どおり空白は除かない
	}
	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			t.Setenv("APP_ENV", tt.env)
			if got := IsProduction(); got != tt.want {
				t.Errorf("APP_ENV=%q: IsProduction = %v, want %v", tt.env, got, tt.want)
			}
		})
	}
}

func TestStringGetters(t *testing.T) {
	tests := []struct {
		name string
		key  string
		get  func() string
		val  string
		want string
	}{
		{"RAG URL は空白を除く", "RAG_REVIEW_URL", RAGReviewURL, "  http://rag:9000 \n", "http://rag:9000"},
		{"RAG URL 未設定は空", "RAG_REVIEW_URL", RAGReviewURL, "", ""},
		{"FRONTEND_URL は空白を除く", "FRONTEND_URL", FrontendURL, " http://fe\n", "http://fe"},
		// 検証側 LoadConfig と同じ値で署名するため、秘密鍵は空白を除かない
		{"USER_SECRET はそのまま", "USER_SECRET", UserSecret, " u\n", " u\n"},
		{"ADMIN_SECRET はそのまま", "ADMIN_SECRET", AdminSecret, " a\n", " a\n"},
		{"TOKEN_ENCRYPTION_KEY はそのまま", "TOKEN_ENCRYPTION_KEY", TokenEncryptionKey, " k\n", " k\n"},
		{"USER_SECRET 未設定は空", "USER_SECRET", UserSecret, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.key, tt.val)
			if got := tt.get(); got != tt.want {
				t.Errorf("%s=%q: got %q, want %q", tt.key, tt.val, got, tt.want)
			}
		})
	}
}
