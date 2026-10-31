package config_test

// SERVICE_SECRET の解決。
//
// .env.example の `change-me-service-secret` は公開されている値なので、
// そのまま本番/stagingへ出ると「誰でも /api/admin/whats-new/ingest と
// /api/admin/training/stats を叩ける」状態になる。EchoStaticSecretAuth は
// 空文字以外を有効な鍵として扱うため、プレースホルダーは config の側で
// 空へ落として 503 fail-close させる(#1158)。
//
// 起動時に落とす(log.Fatalf)形は採らない。Terraform apply より先に新しい
// イメージがデプロイされた時点で本番が起動不能になるため。

import (
	"testing"

	"Backend/internal/config"
)

func TestServiceSecret_プレースホルダーは未設定扱い(t *testing.T) {
	tests := []struct {
		name  string
		env   string
		value string
		want  string
	}{
		{"正しい値はそのまま使う", "development", "real-service-secret", "real-service-secret"},
		{"プレースホルダーは空へ落とす（開発）", "development", "change-me-service-secret", ""},
		{"プレースホルダーは空へ落とす（本番）", "production", "change-me-service-secret", ""},
		{"未設定は空のまま", "development", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDBEnv(t)
			// 本番扱いでも必須チェックに引っかからないよう他のシークレットを埋める。
			t.Setenv("APP_ENV", tt.env)
			t.Setenv("ADMIN_SECRET", "admin-hmac-key")
			t.Setenv("USER_SECRET", "user-secret-value")
			t.Setenv("COMPANY_USER_SECRET", "company-user-secret-value")
			t.Setenv("SERVICE_SECRET", tt.value)

			cfg, err := config.LoadConfig()
			if err != nil {
				t.Fatalf("設定を読めない: %v", err)
			}
			if cfg.ServiceSecret != tt.want {
				t.Errorf("ServiceSecret が違う: got %q, want %q", cfg.ServiceSecret, tt.want)
			}
		})
	}
}

// ADMIN_SECRET と同じ値をそのまま使わないこと。兼用へ戻ると分離の意味が無くなる(#1158)。
func TestServiceSecret_ADMIN_SECRETと同値なら使わない(t *testing.T) {
	setDBEnv(t)
	t.Setenv("APP_ENV", "development")
	t.Setenv("ADMIN_SECRET", "same-value")
	t.Setenv("SERVICE_SECRET", "same-value")

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("設定を読めない: %v", err)
	}
	if cfg.ServiceSecret != "" {
		t.Errorf("ADMIN_SECRET と同値なら空にすること: got %q", cfg.ServiceSecret)
	}
}
