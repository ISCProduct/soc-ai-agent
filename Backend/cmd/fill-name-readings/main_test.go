package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAllowed(t *testing.T) {
	tests := []struct {
		name      string
		allowProd bool
		host, env string
		wantErr   bool
	}{
		{"localhost", false, "localhost", "development", false},
		{"127.0.0.1", false, "127.0.0.1", "", false},
		{"本番の環境変数", false, "127.0.0.1", "production", true},
		{"ローカルでないホスト", false, "db.example.internal", "", true},
		{"--allow-production なら許す", true, "db.example.internal", "production", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeAllowed(tt.allowProd, tt.host, tt.env); (err != nil) != tt.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestWriteBackup_RoundTripAndNeverOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "before.json")
	rows := []backupRow{{CompanyID: 7, Reading: "トヨタジドウシャ", Method: "corporate_number"}}
	if err := writeBackup(path, rows); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	var got []backupRow
	if err := json.Unmarshal(b, &got); err != nil || len(got) != 1 || got[0] != rows[0] {
		t.Fatalf("復元に使う値が残る: %v %+v", err, got)
	}
	if err := writeBackup(path, rows); err == nil {
		t.Fatal("既存のバックアップは上書きしない")
	}
}

func TestSourceOf(t *testing.T) {
	if sourceOf("website") != "website" || sourceOf("gbizinfo") != "gbizinfo" || sourceOf("corporate_number") != "corporate_registry" || sourceOf("name_match") != "corporate_registry" {
		t.Fatal("出どころの対応")
	}
}
