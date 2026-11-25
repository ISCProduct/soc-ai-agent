package config_test

import (
	"testing"

	"Backend/internal/config"
)

// EDINET の設定は config から読む（SOCAIAGENT-726）。キーが空なら取り込みを行わない前提なので、
// 未設定で空、接続先は既定値になることを固定する。
func TestEDINETConfig(t *testing.T) {
	tests := []struct {
		name, key, baseURL   string
		wantKey, wantBaseURL string
	}{
		{name: "未設定", wantBaseURL: "https://api.edinet-fsa.go.jp/api/v2"},
		{name: "設定あり", key: "dummy-key", baseURL: "http://localhost:9999/api/v2", wantKey: "dummy-key", wantBaseURL: "http://localhost:9999/api/v2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDBEnv(t)
			t.Setenv("EDINET_API_KEY", tt.key)
			t.Setenv("EDINET_BASE_URL", tt.baseURL)
			cfg, err := config.LoadConfig()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.EDINETAPIKey != tt.wantKey || cfg.EDINETBaseURL != tt.wantBaseURL {
				t.Errorf("got key=%q base=%q, want key=%q base=%q", cfg.EDINETAPIKey, cfg.EDINETBaseURL, tt.wantKey, tt.wantBaseURL)
			}
		})
	}
}
