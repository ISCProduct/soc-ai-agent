package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckWriteTarget(t *testing.T) {
	tests := []struct {
		name, env, host string
		wantErr         bool
	}{
		{"ローカル開発は許す", "development", "localhost", false},
		{"APP_ENV未設定のloopbackは許す", "", "127.0.0.1", false},
		{"本番は拒否", "production", "localhost", true},
		{"大文字の本番も拒否", "Production", "localhost", true},
		{"prodも拒否", "prod", "localhost", true},
		{"ローカル以外のホストは拒否", "development", "db.internal.example", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkWriteTarget(tt.env, tt.host); (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWriteNewFileDoesNotOverwrite(t *testing.T) {
	p := filepath.Join(t.TempDir(), "before.json")
	if err := writeNewFile(p, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := writeNewFile(p, []byte("second")); err == nil {
		t.Fatal("既存ファイルへの書き込みは失敗するべき")
	}
	if b, _ := os.ReadFile(p); string(b) != "first" {
		t.Errorf("既存の内容が変わった: %q", b)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
		t.Errorf("権限 = %v, want 0600", fi.Mode().Perm())
	}
}
