package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

// 空の backup は「戻す対象がある」入力として成立しない。
// --apply は修正0件のとき backup を作らないので、null / [] は作られ得ない形。
// 受け付けると ApplyAuditChanges が何もせず成功し、「戻した」と誤解する。
func TestParseBackup(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantN   int
		wantErr string
	}{
		{"1件の backup を読める", `[{"id":7,"name":"C社","before":{"industry":"情報通信業"},"after":{"industry":"IT・ソフトウェア"}}]`, 1, ""},
		{"null は拒否", `null`, 0, "復元対象がありません"},
		{"空配列は拒否", `[]`, 0, "復元対象がありません"},
		{"壊れた JSON は拒否", `{`, 0, "形式が不正"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBackup([]byte(tt.json))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q を含む", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tt.wantN {
				t.Errorf("件数 = %d, want %d", len(got), tt.wantN)
			}
		})
	}
}

// JSON の数値は float64 で入るので、整数列へ渡す前に整数へ戻す。
// float64 のままだと CAS 条件が "1990" ではなく "1990.0" 相当になり一致しない。
func TestParseBackupConvertsNumbersToInt(t *testing.T) {
	got, err := parseBackup([]byte(`[{"id":1,"name":"A","before":{"founded_year":1990},"after":{"founded_year":0}}]`))
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := got[0].Before["founded_year"].(int); !ok || v != 1990 {
		t.Errorf("before.founded_year = %#v, want int(1990)", got[0].Before["founded_year"])
	}
	if v, ok := got[0].After["founded_year"].(int); !ok || v != 0 {
		t.Errorf("after.founded_year = %#v, want int(0)", got[0].After["founded_year"])
	}
}

// DB_HOST=::1 のとき "::1:3306" を作ると、ドライバがこれを正しい IPv6 の
// 接続先へ戻さず接続に失敗する。角括弧で包む必要がある。
func TestDBAddr(t *testing.T) {
	tests := []struct{ name, host, port, want string }{
		{"ホスト名", "localhost", "3306", "localhost:3306"},
		{"IPv4", "127.0.0.1", "3306", "127.0.0.1:3306"},
		{"IPv6 は角括弧で包む", "::1", "3306", "[::1]:3306"},
		{"IPv6(完全表記)も包む", "2001:db8::1", "13306", "[2001:db8::1]:13306"},
		{"既定値", "", "", "localhost:3306"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DB_HOST", tt.host)
			t.Setenv("DB_PORT", tt.port)
			if got := dbAddr(); got != tt.want {
				t.Errorf("dbAddr() = %q, want %q", got, tt.want)
			}
		})
	}
}

// 論理削除(is_active=false)された企業は点検・修正の対象にしない。
// 条件が無いと、消した企業まで分布・修正計画・--apply の対象になる。
// wiki も「論理削除された企業は対象外」と書いているので、実装をそれに合わせる。
func TestLoadAuditTargetsExcludesInactive(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery(regexp.QuoteMeta("WHERE is_active = ?")).
		WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "A社"))

	got, err := loadAuditTargets(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "A社" {
		t.Errorf("読み込み結果 = %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}
