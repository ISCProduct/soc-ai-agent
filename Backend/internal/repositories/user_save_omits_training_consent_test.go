package repositories

import (
	"strings"
	"sync"
	"testing"
	"time"

	"Backend/domain/entity"
	"Backend/internal/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// #2262 のレビュー: users を全カラムで保存する経路（UpdateUser・退会の Withdraw）は、
// 学習利用の同意（allow_training_use）を書かない。読み込んだ時点の古い値で書き戻すと、
// その間に本人が取り消した同意が元に戻り、履歴（training_consent_events）とも食い違う。
//
// 発行される SQL をすべて記録し、users の UPDATE に allow_training_use が無いことと、
// ほかのカラム（allow_scout_visibility など）は従来どおり保存していることを確かめる。

// sqlRecorder は、どの SQL にも当てはまる期待を置き、実行された SQL を記録する。
// 狙った文（users の UPDATE）の中身だけを見たいので、ほかの文の並びには依存しない。
type sqlRecorder struct {
	mu   sync.Mutex
	sqls []string
}

func (r *sqlRecorder) match(_, actual string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sqls = append(r.sqls, actual)
	return nil
}

func (r *sqlRecorder) find(prefix string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var got []string
	for _, s := range r.sqls {
		if strings.HasPrefix(s, prefix) {
			got = append(got, s)
		}
	}
	return got
}

func newRecordingDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, *sqlRecorder) {
	t.Helper()
	rec := &sqlRecorder{}
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(rec.match)))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}
	return db, mock, rec
}

// assertUserUpdateOmitsTrainingConsent は、users の UPDATE が1本以上あり、どれも allow_training_use を
// 書かず、allow_scout_visibility（同じく全カラム保存で書かれる同意）は書いていることを確かめる。
// 後者で「UPDATE が別の形（対象カラムだけ）に変わったので当たらない」誤検知を防ぐ。
func assertUserUpdateOmitsTrainingConsent(t *testing.T, rec *sqlRecorder) {
	t.Helper()
	updates := rec.find("UPDATE `users` SET")
	if len(updates) == 0 {
		t.Fatalf("users の UPDATE が実行されていない: %v", rec.sqls)
	}
	for _, s := range updates {
		if strings.Contains(s, "`allow_training_use`") {
			t.Errorf("users の全カラム保存が allow_training_use を書いている: %s", s)
		}
		if !strings.Contains(s, "`allow_scout_visibility`") {
			t.Errorf("全カラム保存のはずが、ほかのカラムを書いていない（確かめ方が成り立たない）: %s", s)
		}
	}
}

func TestUpdateUser_DoesNotWriteTrainingConsent(t *testing.T) {
	db, mock, rec := newRecordingDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	// 読み込んだ時点では同意済み（true）。この値で書き戻すと、その後の取り消しが消える。
	err := NewUserRepository(db).UpdateUser(&entity.User{ID: 9, Email: "a@example.com", Name: "a", AllowTrainingUse: true})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	assertUserUpdateOmitsTrainingConsent(t, rec)
}

func TestWithdraw_DoesNotWriteTrainingConsent(t *testing.T) {
	db, mock, rec := newRecordingDB(t)
	mock.MatchExpectationsInOrder(false)
	mock.ExpectBegin()
	// 最初の SELECT（users の取得）。順不同なので、どの SELECT もこの行か空の行を返す。
	mock.ExpectQuery("users").WillReturnRows(
		sqlmock.NewRows([]string{"id", "email", "allow_training_use", "allow_scout_visibility"}).
			AddRow(9, "a@example.com", true, true))
	for range 10 {
		mock.ExpectQuery("any").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	}
	for range 10 {
		mock.ExpectExec("any").WillReturnResult(sqlmock.NewResult(1, 1))
	}
	mock.ExpectCommit()

	build := func(u *models.User, _ []string) models.WithdrawnUser {
		return models.WithdrawnUser{UserID: u.ID, PurgeAfter: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)}
	}
	if err := NewUserWithdrawalRepository(db).Withdraw(9, time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC), build); err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	assertUserUpdateOmitsTrainingConsent(t, rec)
}
