package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"Backend/internal/models"
)

func TestWriteAllowed(t *testing.T) {
	tests := []struct {
		name      string
		allowProd bool
		host      string
		env       string
		wantErr   bool
	}{
		{"localhost", false, "localhost", "development", false},
		{"127.0.0.1", false, "127.0.0.1", "", false},
		{"::1", false, "::1", "", false},
		{"本番の環境変数", false, "127.0.0.1", "production", true},
		{"ローカルでないホスト", false, "db.example.internal", "", true},
		{"ローカルでないIP", false, "10.0.0.5", "", true},
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
	ps := []models.CompanyWeightProfile{
		{CompanyID: 7, TechnicalOrientation: 100, TeamworkOrientation: 99, LeadershipOrientation: 10, CreativityOrientation: 20,
			StabilityOrientation: 30, GrowthOrientation: 40, WorkLifeBalance: 50, ChallengeSeeking: 60, DetailOrientation: 70, CommunicationSkill: 80},
	}
	if err := writeBackup(path, ps); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	var rows []backupRow
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].CompanyID != 7 || rows[0].Axes != [10]int{100, 99, 10, 20, 30, 40, 50, 60, 70, 80} {
		t.Fatalf("復元に使う値が、軸の並びどおりに残る: %+v", rows)
	}
	if err := writeBackup(path, ps); err == nil {
		t.Fatal("既存のバックアップは上書きしない")
	}
}
