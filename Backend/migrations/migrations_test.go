package migrations

import (
	"regexp"
	"strconv"
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

// TestMigrationVersionsAreContiguous はマイグレーション番号に穴が無いことを検証する（#929）。
//
// golang-migrate の Up() は現在バージョンから Next() で前進するだけなので、
// 適用済みバージョンより小さい番号のマイグレーションが後から追加されても
// 一度も実行されず ErrNoChange で正常終了する。エラーもログも出ない。
//
// 実際に、別ブランチが 000019 を使っている状態で 000020 を先にマージすると
// 000019 が本番へ永久に適用されない、という事故が起きかけた。
// 番号が飛んだ時点でCIを落として気づけるようにする。
func TestMigrationVersionsAreContiguous(t *testing.T) {
	entries, err := migrationFiles.ReadDir(".")
	if err != nil {
		t.Fatalf("マイグレーションディレクトリを読めない: %v", err)
	}

	versions := map[int]string{}
	re := regexp.MustCompile(`^(\d+)_.*\.(up|down)\.sql$`)
	for _, e := range entries {
		m := re.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("番号を解釈できない: %s", e.Name())
		}
		versions[v] = e.Name()
	}
	if len(versions) == 0 {
		t.Fatal("マイグレーションが1つも見つからない")
	}

	max := 0
	for v := range versions {
		if v > max {
			max = v
		}
	}
	// 1 から最大値まで、欠番が無いこと。
	for v := 1; v <= max; v++ {
		if _, ok := versions[v]; !ok {
			t.Errorf("マイグレーション %06d が欠番。golang-migrate は適用済みより小さい番号を"+
				"二度と実行しないため、後から埋めても本番へ適用されない", v)
		}
	}

	// up と down が対になっていること。
	for v, name := range versions {
		base := strings.TrimSuffix(strings.TrimSuffix(name, ".up.sql"), ".down.sql")
		for _, suffix := range []string{".up.sql", ".down.sql"} {
			if _, err := migrationFiles.ReadFile(base + suffix); err != nil {
				t.Errorf("%06d: %s が無い", v, base+suffix)
			}
		}
	}
}
