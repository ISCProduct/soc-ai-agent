package company

import (
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}
	return db, mock
}

func TestApplyAuditChanges(t *testing.T) {
	change := []AuditChange{{
		ID: 7, Name: "C社",
		Before: map[string]any{"industry": "情報通信業"},
		After:  map[string]any{"industry": "IT・ソフトウェア"},
	}}

	t.Run("適用は現在値が before のままの行だけを更新する", func(t *testing.T) {
		db, mock := newMockDB(t)
		mock.ExpectExec(regexp.QuoteMeta("UPDATE `companies` SET `industry`=? WHERE id = ? AND CAST(`industry` AS BINARY) = CAST(? AS BINARY)")).
			WithArgs("IT・ソフトウェア", uint(7), "情報通信業").
			WillReturnResult(sqlmock.NewResult(0, 1))
		if err := ApplyAuditChanges(db, change, false); err != nil {
			t.Fatal(err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
	})

	t.Run("読み込み後に値が変わっていたら更新せず失敗する", func(t *testing.T) {
		db, mock := newMockDB(t)
		mock.ExpectExec("UPDATE `companies`").WillReturnResult(sqlmock.NewResult(0, 0))
		err := ApplyAuditChanges(db, change, false)
		if err == nil || !strings.Contains(err.Error(), "想定と違う") {
			t.Fatalf("err = %v, want 想定と違う", err)
		}
	})

	t.Run("戻しは after の行を before へ戻す", func(t *testing.T) {
		db, mock := newMockDB(t)
		mock.ExpectExec(regexp.QuoteMeta("UPDATE `companies` SET `industry`=? WHERE id = ? AND CAST(`industry` AS BINARY) = CAST(? AS BINARY)")).
			WithArgs("情報通信業", uint(7), "IT・ソフトウェア").
			WillReturnResult(sqlmock.NewResult(0, 1))
		if err := ApplyAuditChanges(db, change, true); err != nil {
			t.Fatal(err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
	})
}

// companies は utf8mb4_unicode_ci。`col = ?` の比較では大文字小文字と末尾空白の違いを
// 取りこぼすため、「適用後に手で直された値は巻き戻さない」が成立しない。
// 例: URL の空白を落としたあと、管理者がパスを /Recruit から /recruit に直しても
// 現在値が適用後の値と同一と判定され、その修正を古い URL で上書きしてしまう。
func TestApplyAuditChangesComparesBytes(t *testing.T) {
	db, mock := newMockDB(t)
	changes := []AuditChange{{
		ID: 7, Name: "C社",
		Before: map[string]any{"website_url": "https://example.com/Recruit "},
		After:  map[string]any{"website_url": "https://example.com/Recruit"},
	}}
	mock.ExpectExec(regexp.QuoteMeta(
		"UPDATE `companies` SET `website_url`=? WHERE id = ? AND CAST(`website_url` AS BINARY) = CAST(? AS BINARY)")).
		WithArgs("https://example.com/Recruit", uint(7), "https://example.com/Recruit ").
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := ApplyAuditChanges(db, changes, false); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

// 数値列はバイト比較の対象ではない(照合順序の影響を受けない)。
func TestApplyAuditChangesKeepsPlainCompareForNumbers(t *testing.T) {
	db, mock := newMockDB(t)
	changes := []AuditChange{{
		ID: 7, Name: "C社",
		Before: map[string]any{"founded_year": 1500},
		After:  map[string]any{"founded_year": 0},
	}}
	mock.ExpectExec(regexp.QuoteMeta(
		"UPDATE `companies` SET `founded_year`=? WHERE id = ? AND `founded_year` = ?")).
		WithArgs(0, uint(7), 1500).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := ApplyAuditChanges(db, changes, false); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

// 列名は SQL へ識別子として埋めるので、点検対象外の名前は実行前に止める。
// --restore は外部の JSON ファイルを読むため、壊れた・細工された backup が入口になる。
func TestApplyAuditChangesRejectsUnknownColumns(t *testing.T) {
	tests := []struct {
		name   string
		change AuditChange
		want   string
	}{
		{
			"before に点検対象外の列",
			AuditChange{ID: 1, Name: "A", Before: map[string]any{"is_active": false}, After: map[string]any{"industry": "x"}},
			"点検対象でない列",
		},
		{
			"after に点検対象外の列",
			AuditChange{ID: 1, Name: "A", Before: map[string]any{"industry": "x"}, After: map[string]any{"password": "y"}},
			"点検対象でない列",
		},
		{
			"SQL を混ぜた列名",
			AuditChange{ID: 1, Name: "A", Before: map[string]any{"industry` = industry OR `1": "x"}, After: map[string]any{"industry": "y"}},
			"点検対象でない列",
		},
		{
			"before が空",
			AuditChange{ID: 1, Name: "A", Before: map[string]any{}, After: map[string]any{"industry": "y"}},
			"空です",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMockDB(t)
			// SQL を1本も発行しないこと（ExpectExec を置かないので、発行すれば失敗する）
			err := ApplyAuditChanges(db, []AuditChange{tt.change}, false)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q を含む", err, tt.want)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Error(err)
			}
		})
	}
}
