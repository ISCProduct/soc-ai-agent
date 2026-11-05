package github

import (
	"Backend/internal/crypto"
	"errors"
	"testing"
)

// StoreAccessToken と同じ方式で保存したトークンを、要約・同期の両経路が読めること
func TestLoadAccessToken(t *testing.T) {
	const plain = "gho_example"
	encrypted, err := crypto.EncryptToken(plain, "test-key")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		key     string
		stored  string
		want    string
		wantErr bool
	}{
		{name: "暗号化保存を TOKEN_ENCRYPTION_KEY で復号", key: "test-key", stored: encrypted, want: plain},
		{name: "鍵未設定なら平文保存をそのまま使う", key: "", stored: plain, want: plain},
		{name: "鍵違いは再連携エラー", key: "other-key", stored: encrypted, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TOKEN_ENCRYPTION_KEY", tt.key)
			got, err := loadAccessToken(1, tt.stored)
			if tt.wantErr {
				var reauth *GitHubReauthRequiredError
				if !errors.As(err, &reauth) {
					t.Fatalf("want GitHubReauthRequiredError, got %v", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}
