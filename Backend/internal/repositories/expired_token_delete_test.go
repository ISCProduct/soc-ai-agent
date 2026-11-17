package repositories

import (
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 期限切れの仮登録・リフレッシュトークンを消す SQL を固定する（#2153）。
// 期限の比較（仮登録は期限ちょうども消す、リフレッシュトークンは before より前だけ）、
// 古い順に上限件数だけ消すこと、削除件数とエラーを呼び出し側に返すことを確かめる。

func newExpiredDeleteTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock作成失敗: %v", err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm open失敗: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db, mock
}

type expiredDeleteFunc func(db *gorm.DB, cutoff time.Time, limit int) (int64, error)

var expiredDeleteCases = []struct {
	name    string
	del     expiredDeleteFunc
	wantSQL string
}{
	{
		name: "仮登録",
		del: func(db *gorm.DB, cutoff time.Time, limit int) (int64, error) {
			return NewPendingRegistrationRepository(db).DeleteExpired(cutoff, limit)
		},
		wantSQL: "DELETE FROM `pending_registrations` WHERE expires_at <= ? ORDER BY expires_at LIMIT ?",
	},
	{
		name: "リフレッシュトークン",
		del: func(db *gorm.DB, cutoff time.Time, limit int) (int64, error) {
			return NewUserRefreshTokenRepository(db).DeleteExpired(cutoff, limit)
		},
		wantSQL: "DELETE FROM `user_refresh_tokens` WHERE expires_at < ? ORDER BY expires_at LIMIT ?",
	},
	{
		name: "企業ユーザーのリフレッシュトークン",
		del: func(db *gorm.DB, cutoff time.Time, limit int) (int64, error) {
			return NewCompanyUserRefreshTokenRepository(db).DeleteExpired(cutoff, limit)
		},
		wantSQL: "DELETE FROM `company_user_refresh_tokens` WHERE expires_at < ? ORDER BY expires_at LIMIT ?",
	},
}

func TestDeleteExpired_古い順に上限件数だけ消して件数を返す(t *testing.T) {
	cutoff := time.Date(2026, 11, 11, 3, 0, 0, 0, time.UTC)
	for _, tc := range expiredDeleteCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newExpiredDeleteTestDB(t)
			// GORM は Delete を既定のトランザクションで包む
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta(tc.wantSQL)).
				WithArgs(cutoff, 1000).
				WillReturnResult(sqlmock.NewResult(0, 7))
			mock.ExpectCommit()

			n, err := tc.del(db, cutoff, 1000)
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if n != 7 {
				t.Errorf("削除件数 = %d, want 7", n)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("SQL が想定と違う: %v", err)
			}
		})
	}
}

func TestDeleteExpired_DBのエラーを返す(t *testing.T) {
	cutoff := time.Date(2026, 11, 11, 3, 0, 0, 0, time.UTC)
	errDB := errors.New("deadlock")
	for _, tc := range expiredDeleteCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newExpiredDeleteTestDB(t)
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta(tc.wantSQL)).
				WithArgs(cutoff, 1).
				WillReturnError(errDB)
			mock.ExpectRollback()

			_, err := tc.del(db, cutoff, 1)
			if !errors.Is(err, errDB) {
				t.Errorf("err = %v, want %v", err, errDB)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("SQL が想定と違う: %v", err)
			}
		})
	}
}

// GORM は Limit(-1) を「上限なし」と解釈する。0 以下を通すと全件を1文で消してしまうので、
// SQL を投げずにエラーにすることを固定する。
func TestDeleteExpired_上限が0以下ならSQLを投げずにエラー(t *testing.T) {
	cutoff := time.Date(2026, 11, 11, 3, 0, 0, 0, time.UTC)
	for _, tc := range expiredDeleteCases {
		for _, limit := range []int{0, -1} {
			t.Run(fmt.Sprintf("%s/limit=%d", tc.name, limit), func(t *testing.T) {
				db, mock := newExpiredDeleteTestDB(t)
				// SQL が投げられたら成功を返す期待を置く。期待が無いと、投げた SQL が
				// sqlmock のエラーになり、上限のチェックで弾いた場合と区別できないため。
				mock.ExpectBegin()
				mock.ExpectExec("DELETE").WillReturnResult(sqlmock.NewResult(0, 5))
				mock.ExpectCommit()

				n, err := tc.del(db, cutoff, limit)
				if err == nil {
					t.Errorf("limit=%d: err = nil, want エラー", limit)
				}
				if n != 0 {
					t.Errorf("limit=%d: 削除件数 = %d, want 0", limit, n)
				}
				if mock.ExpectationsWereMet() == nil {
					t.Errorf("limit=%d: SQL が投げられた", limit)
				}
			})
		}
	}
}
