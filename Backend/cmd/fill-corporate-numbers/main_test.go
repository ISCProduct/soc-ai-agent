package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"Backend/internal/models"
	"Backend/internal/services/houjinbangou"
)

// fakeFinder は商号ごとに決めた結果を返す（外部ネットワークは呼ばない）。
type fakeFinder struct {
	byName map[string]*houjinbangou.Corporation
	errs   map[string]error
}

func (f fakeFinder) ResolveCorporateNumber(_ context.Context, name, _ string) (*houjinbangou.Corporation, error) {
	return f.byName[name], f.errs[name]
}

func TestResolveAll(t *testing.T) {
	f := fakeFinder{
		byName: map[string]*houjinbangou.Corporation{
			"一意":  {CorporateNumber: "1234567890123"},
			"閉鎖":  {CorporateNumber: "2234567890123", CloseDate: "2020-01-01"},
			"桁不正": {CorporateNumber: "123"},
			// "複数・該当なし" は nil（既存関数は理由を返さない）
		},
		errs: map[string]error{"通信失敗": errors.New("boom")},
	}
	targets := []models.Company{
		{ID: 1, Name: "一意"}, {ID: 2, Name: "複数・該当なし"}, {ID: 3, Name: "閉鎖"},
		{ID: 4, Name: "桁不正"}, {ID: 5, Name: "通信失敗"},
	}
	got := resolveAll(context.Background(), f, targets)

	if len(got.Found) != 1 || got.Found[0] != (backupRow{1, "1234567890123"}) {
		t.Fatalf("特定は一意のみ: %+v", got.Found)
	}
	want := map[uint]string{2: reasonNotResolved, 3: reasonClosed, 4: reasonBadNumber}
	if len(got.Misses) != 3 {
		t.Fatalf("採用しない3社: %+v", got.Misses)
	}
	for _, m := range got.Misses {
		if want[m.ID] != m.Reason {
			t.Errorf("企業 %d の理由 = %q, want %q", m.ID, m.Reason, want[m.ID])
		}
	}
	if got.Failed != 1 {
		t.Fatalf("通信失敗は1社: %d", got.Failed)
	}
}

// 既存の法人番号を変えないのは、SQL の条件（対象選択と書き込みの両方）で担保する。DB には接続せず、生成される SQL を検査する。
func dryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	sqlDB, err := sql.Open("mysql", "u:p@tcp(127.0.0.1:1)/x")
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestTargetQuery(t *testing.T) {
	db := dryRunDB(t)
	tests := []struct {
		name          string
		includeDrafts bool
		limit         int
		wantPublished bool
		wantLimit     bool
	}{
		{"既定は公開済みだけ", false, 0, true, false},
		{"--include-drafts は data_status を問わない", true, 0, false, false},
		{"--limit", false, 5, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cs []models.Company
			stmt := targetQuery(db, tt.includeDrafts, tt.limit).Find(&cs).Statement
			q := stmt.SQL.String()
			for _, must := range []string{"is_active", "is_guest_entry", "corporate_number IS NULL OR corporate_number = ''"} {
				if !strings.Contains(q, must) {
					t.Errorf("条件 %q が無い: %s", must, q)
				}
			}
			if got := strings.Contains(q, "data_status"); got != tt.wantPublished {
				t.Errorf("data_status 条件 = %v, want %v: %s", got, tt.wantPublished, q)
			}
			if got := strings.Contains(q, "LIMIT"); got != tt.wantLimit {
				t.Errorf("LIMIT = %v, want %v: %s", got, tt.wantLimit, q)
			}
		})
	}
}

func TestBackup_RoundTripAndNeverOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "before.json")
	rows := []backupRow{{CompanyID: 7, CorporateNumber: "1234567890123"}}
	if err := writeBackup(path, rows); err != nil {
		t.Fatal(err)
	}
	got, err := readBackup(path)
	if err != nil || len(got) != 1 || got[0] != rows[0] {
		t.Fatalf("復元に使う値が残る: %v %+v", err, got)
	}
	if err := writeBackup(path, rows); err == nil {
		t.Fatal("既存のバックアップは上書きしない")
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	_ = os.WriteFile(bad, []byte("x"), 0o600)
	if _, err := readBackup(bad); err == nil {
		t.Fatal("不正な形式は読めない")
	}
}

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

func TestAppIDUnset(t *testing.T) {
	if houjinbangou.NewClient("", "").Enabled() {
		t.Fatal("ID が空なら無効（main はここで止まる）")
	}
}

func TestFillAndRestoreSQL(t *testing.T) {
	db := dryRunDB(t)
	r := backupRow{CompanyID: 7, CorporateNumber: "1234567890123"}

	fill := fillOne(db, r).Statement.SQL.String()
	if !strings.Contains(fill, "corporate_number IS NULL OR corporate_number = ''") {
		t.Errorf("既存値を変えない条件が無い: %s", fill)
	}
	set := fill[:strings.Index(fill, "WHERE")]
	if strings.Count(set, "=") != 1 || !strings.Contains(set, "corporate_number") {
		t.Errorf("corporate_number 以外を更新している: %s", set)
	}

	rest := restoreOne(db, r).Statement.SQL.String()
	if !strings.Contains(rest, "corporate_number = ?") || !strings.Contains(rest, "id = ?") {
		t.Errorf("書き込んだ値のままのときだけ戻す条件が無い: %s", rest)
	}
}
