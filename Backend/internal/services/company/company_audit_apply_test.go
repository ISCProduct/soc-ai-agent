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
		mock.ExpectExec(regexp.QuoteMeta("UPDATE `companies` SET `industry`=? WHERE id = ? AND `companies`.`industry` = ?")).
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
		mock.ExpectExec(regexp.QuoteMeta("UPDATE `companies` SET `industry`=? WHERE id = ? AND `companies`.`industry` = ?")).
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
