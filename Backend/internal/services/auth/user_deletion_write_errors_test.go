package auth

import (
	"Backend/internal/repositories"
	"bytes"
	"database/sql/driver"
	"errors"
	"log"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"Backend/internal/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// #2001: 物理パージ（hardDeleteUserData / purgeOne）で書き込みエラーを握りつぶさない。
//
// パージは削除文が30近く続くので、文を順に並べると、削除するテーブルを足すたびにテストが壊れる。
// そこで、狙った1本だけを差し替え、それ以外の SQL は全部通して記録する。

const anySQL = "<any>"

// purgeTestDB は、anySQL の期待にはどの SQL も当てはまり、実行された SQL を記録する DB を返す。
// anySQL 以外の期待は、その文字列を含む SQL にだけ当てはまる（失敗させたい1本を指定する）。
type purgeTestDB struct {
	db   *gorm.DB
	mock sqlmock.Sqlmock
	mu   sync.Mutex
	sqls []string
}

func newPurgeTestDB(t *testing.T) *purgeTestDB {
	t.Helper()
	p := &purgeTestDB{}
	matcher := sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		if expected == anySQL {
			p.mu.Lock()
			p.sqls = append(p.sqls, actual)
			p.mu.Unlock()
			return nil
		}
		if strings.Contains(actual, expected) {
			return nil
		}
		return errors.New("no match")
	})
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	mock.MatchExpectationsInOrder(false)
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}
	p.db, p.mock = db, mock
	return p
}

// expectUser は、パージ対象のユーザーの取得を返す（先に登録して anySQL より優先させる）。
func (p *purgeTestDB) expectUser(id uint) {
	p.mock.ExpectQuery("FROM `users` WHERE `users`.`id` =").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).AddRow(id, "withdrawn+9@example.invalid"))
}

// allowRest は、残りの SELECT・DELETE・UPDATE を全部通す（多めに登録しておく）。
func (p *purgeTestDB) allowRest() {
	for range 40 {
		p.mock.ExpectQuery(anySQL).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	}
	for range 60 {
		p.mock.ExpectExec(anySQL).WillReturnResult(sqlmock.NewResult(0, 0))
	}
}

func (p *purgeTestDB) executed(substr string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.sqls {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

func capturePurgeLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return &buf
}

var purgeTarget = models.WithdrawnUser{ID: 5, UserID: 9, PurgeAfter: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}

func TestPurgeOne_PendingRegistrationDeleteFails_RollsBack(t *testing.T) {
	p := newPurgeTestDB(t)
	p.mock.ExpectBegin()
	p.expectUser(9)
	p.mock.ExpectExec("DELETE FROM `pending_registrations`").WillReturnError(errors.New("db down"))
	p.mock.ExpectRollback()
	p.allowRest()

	err := NewUserDeletionService(repositories.NewUserWithdrawalRepository(p.db), nil, nil).purgeOne(purgeTarget)

	if err == nil || !strings.Contains(err.Error(), "db down") {
		t.Fatalf("仮登録を消せなかったので失敗を返すべき: err=%v", err)
	}
	if p.executed("DELETE FROM `users`") {
		t.Error("仮登録の削除に失敗した後で、ユーザーを削除した（巻き戻すべき）")
	}
	if p.executed("UPDATE `withdrawn_users`") {
		t.Error("パージに失敗したのに purged_at を記録した（次回の再試行の対象から外れる）")
	}
}

func TestPurgeOne_Normal_DeletesUserAndRecordsPurgedAt(t *testing.T) {
	buf := capturePurgeLog(t)
	p := newPurgeTestDB(t)
	p.mock.ExpectBegin()
	p.expectUser(9)
	p.mock.ExpectCommit()
	p.mock.ExpectBegin()
	p.mock.ExpectCommit()
	p.allowRest()

	if err := NewUserDeletionService(repositories.NewUserWithdrawalRepository(p.db), nil, nil).purgeOne(purgeTarget); err != nil {
		t.Fatalf("purgeOne: %v", err)
	}
	for _, want := range []string{"DELETE FROM `pending_registrations`", "DELETE FROM `users`", "UPDATE `withdrawn_users`"} {
		if !p.executed(want) {
			t.Errorf("%s が実行されていない", want)
		}
	}
	if strings.Contains(buf.String(), "[UserDeletion]") {
		t.Errorf("通常時に余計なログが出た: %q", buf.String())
	}
}

func TestPurgeOne_PurgedAtUpdateFails_StillSucceedsAndLogged(t *testing.T) {
	buf := capturePurgeLog(t)
	p := newPurgeTestDB(t)
	p.mock.ExpectBegin()
	p.expectUser(9)
	p.mock.ExpectCommit()
	p.mock.ExpectBegin()
	p.mock.ExpectExec("UPDATE `withdrawn_users`").WillReturnError(errors.New("db down"))
	p.mock.ExpectRollback()
	p.allowRest()

	if err := NewUserDeletionService(repositories.NewUserWithdrawalRepository(p.db), nil, nil).purgeOne(purgeTarget); err != nil {
		t.Fatalf("物理削除はコミット済みなので成功を返すべき: %v", err)
	}
	if !p.executed("DELETE FROM `users`") {
		t.Error("ユーザーが削除されていない")
	}
	if got := buf.String(); !strings.Contains(got, "failed to record purged_at withdrawn_user_id=5 user_id=9: db down") {
		t.Errorf("purged_at を記録できなかったことがログに出ていない: %q", got)
	}
}

// #2140 のレビュー: 退会（論理削除）のとき、スクランブルする前に元のメールアドレスの仮登録を消す。
// 消せなければ退会全体を巻き戻し、メールのスクランブルも行わない。
func TestDeleteUser_PendingRegistrationDeleteFails_RollsBack(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE `users`.`id` = \\?").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "withdrawn_at"}).AddRow(9, "del@example.com", nil))
	mock.ExpectQuery("SELECT \\* FROM `interview_videos`").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT \\* FROM `resume_documents`").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec("INSERT INTO `withdrawn_users`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM `pending_registrations` WHERE email = \\?").
		WithArgs("del@example.com").
		WillReturnError(errors.New("db down"))
	mock.ExpectRollback()

	err = NewUserDeletionService(repositories.NewUserWithdrawalRepository(db), nil, nil).DeleteUser(9, UserDeletionActor{Kind: "self"})

	if err == nil || !strings.Contains(err.Error(), "delete pending registrations") {
		t.Fatalf("仮登録を消せなかったので退会を失敗として返すべき: %v", err)
	}
	// UPDATE `users`（スクランブル）が来れば、期待に無い呼び出しとしてここで分かる
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected DB calls: %v", err)
	}
}

// #2168: 物理パージで、学習利用の同意の履歴（training_consent_events）も本人の分を消す。
// 消せなければ全体を巻き戻し、ユーザーも消さない（次回のパージで再試行する）。
func TestPurgeOne_DeletesTrainingConsentEvents(t *testing.T) {
	const deleteEvents = "DELETE FROM `training_consent_events` WHERE user_id = ?"

	t.Run("消す", func(t *testing.T) {
		p := newPurgeTestDB(t)
		p.mock.ExpectBegin()
		p.expectUser(9)
		var deletedFor []driver.Value
		p.mock.ExpectExec(deleteEvents).WithArgs(recordArg{&deletedFor}).WillReturnResult(sqlmock.NewResult(0, 2))
		p.mock.ExpectCommit()
		p.mock.ExpectBegin()
		p.mock.ExpectCommit()
		p.allowRest()

		if err := NewUserDeletionService(repositories.NewUserWithdrawalRepository(p.db), nil, nil).purgeOne(purgeTarget); err != nil {
			t.Fatalf("purgeOne: %v", err)
		}
		// allowRest の期待は余るので ExpectationsWereMet では確かめられない。削除の引数を記録して確かめる
		// sqlmock は照合のたびに Match を呼ぶので、回数ではなく「呼ばれたこと」と値を見る
		if len(deletedFor) == 0 || slices.ContainsFunc(deletedFor, func(v driver.Value) bool { return v != int64(purgeTarget.UserID) }) {
			t.Errorf("同意の履歴を本人（user_id=%d）の分を削除するべき: %v", purgeTarget.UserID, deletedFor)
		}
	})

	t.Run("消せなければ巻き戻す", func(t *testing.T) {
		p := newPurgeTestDB(t)
		p.mock.ExpectBegin()
		p.expectUser(9)
		p.mock.ExpectExec(deleteEvents).WillReturnError(errors.New("db down"))
		p.mock.ExpectRollback()
		p.allowRest()

		if err := NewUserDeletionService(repositories.NewUserWithdrawalRepository(p.db), nil, nil).purgeOne(purgeTarget); err == nil {
			t.Fatal("同意の履歴を消せなかったので失敗を返すべき")
		}
		if p.executed("DELETE FROM `users`") {
			t.Error("同意の履歴の削除に失敗した後で、ユーザーを削除した（巻き戻すべき）")
		}
		if p.executed("UPDATE `withdrawn_users`") {
			t.Error("パージに失敗したのに purged_at を記録した（次回の再試行の対象から外れる）")
		}
	})
}

// recordArg は、どの値にも当てはまり、渡された値を記録する（anySQL と同じく、狙った1本の引数を後から確かめる）。
type recordArg struct{ got *[]driver.Value }

func (a recordArg) Match(v driver.Value) bool {
	*a.got = append(*a.got, v)
	return true
}
