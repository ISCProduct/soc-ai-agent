package migrations

import (
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// TestMigrationFiles はマイグレーションファイルが正しく埋め込まれ、
// up/down がペアで存在することを検証するテーブル駆動テスト
func TestMigrationFiles(t *testing.T) {
	tests := []struct {
		name string
		file string
	}{
		{name: "初期スキーマ up", file: "000001_init_schema.up.sql"},
		{name: "初期スキーマ down", file: "000001_init_schema.down.sql"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := migrationFiles.ReadFile(tt.file)
			if err != nil {
				t.Fatalf("マイグレーションファイルが埋め込まれていない: %v", err)
			}
			if len(data) == 0 {
				t.Fatal("マイグレーションファイルが空")
			}
		})
	}
}

// TestMigrationSourceParsable は golang-migrate がソースとして解釈できることを検証する
func TestMigrationSourceParsable(t *testing.T) {
	src, err := iofs.New(migrationFiles, ".")
	if err != nil {
		t.Fatalf("iofs ソースの生成に失敗: %v", err)
	}
	defer src.Close()

	first, err := src.First()
	if err != nil {
		t.Fatalf("最初のマイグレーションの取得に失敗: %v", err)
	}
	if first != baselineVersion {
		t.Errorf("最初のバージョン = %d, want %d", first, baselineVersion)
	}
}

// TestMigrationPairs は全ての up に対応する down が存在することを検証する
func TestMigrationPairs(t *testing.T) {
	entries, err := migrationFiles.ReadDir(".")
	if err != nil {
		t.Fatalf("埋め込みディレクトリの読み込みに失敗: %v", err)
	}

	ups := map[string]bool{}
	downs := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		switch {
		case strings.HasSuffix(name, ".up.sql"):
			ups[strings.TrimSuffix(name, ".up.sql")] = true
		case strings.HasSuffix(name, ".down.sql"):
			downs[strings.TrimSuffix(name, ".down.sql")] = true
		}
	}

	if len(ups) == 0 {
		t.Fatal("up マイグレーションが1つも存在しない")
	}
	for base := range ups {
		if !downs[base] {
			t.Errorf("%s に対応する down マイグレーションが存在しない", base)
		}
	}
	for base := range downs {
		if !ups[base] {
			t.Errorf("%s に対応する up マイグレーションが存在しない", base)
		}
	}
}
