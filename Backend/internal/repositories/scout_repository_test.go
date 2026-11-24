package repositories

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/internal/models"
)

func newScoutRepoTestDB(t *testing.T) (*ScoutRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock作成失敗: %v", err)
	}
	dialector := mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	})
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm open失敗: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return NewScoutRepository(db), mock
}

func expectScoutSendLock(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `scout_send_locks`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT \\* FROM `scout_send_locks`.*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"company_id", "user_id"}).AddRow(1, 101))
}

// TestCreateScoutWithinCooldown_直列化してから保存する は、
// クールダウン判定と INSERT が同じトランザクションで、ロック行の FOR UPDATE より後に走ることを固定する。
// 判定だけ先に外へ出すと、同時送信の両方が履歴なしと読んで二重に保存される。
func TestCreateScoutWithinCooldown_直列化してから保存する(t *testing.T) {
	repo, mock := newScoutRepoTestDB(t)
	expectScoutSendLock(mock)
	mock.ExpectQuery("SELECT \\* FROM `scouts`.*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec("INSERT INTO `scouts`").
		WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectCommit()

	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	scout := &models.Scout{
		CompanyID: 1,
		UserID:    101,
		SentBy:    9,
		Message:   "本文",
		Status:    models.ScoutStatusSent,
	}
	remaining, err := repo.CreateScoutWithinCooldown(scout, 24*time.Hour, now)
	if err != nil {
		t.Fatalf("保存に失敗: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("remaining=%s", remaining)
	}
	if scout.ID != 7 {
		t.Fatalf("id=%d", scout.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQLが直列化されていない: %v", err)
	}
}

// TestCreateScoutWithinCooldown_クールダウン中は保存しない は、
// ロックを取ったあとの直近行が24時間未満なら INSERT せず残り時間を返すことを固定する。
func TestCreateScoutWithinCooldown_クールダウン中は保存しない(t *testing.T) {
	repo, mock := newScoutRepoTestDB(t)
	expectScoutSendLock(mock)
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	last := now.Add(-time.Hour)
	mock.ExpectQuery("SELECT \\* FROM `scouts`.*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "company_id", "user_id", "template_id", "sent_by", "message", "status", "created_at", "updated_at",
		}).AddRow(3, 1, 101, nil, 9, "前回", models.ScoutStatusSent, last, last))
	mock.ExpectCommit()

	scout := &models.Scout{
		CompanyID: 1,
		UserID:    101,
		SentBy:    9,
		Message:   "本文",
		Status:    models.ScoutStatusSent,
	}
	remaining, err := repo.CreateScoutWithinCooldown(scout, 24*time.Hour, now)
	if err != nil {
		t.Fatalf("クールダウン判定に失敗: %v", err)
	}
	if remaining <= 0 || remaining > 24*time.Hour {
		t.Fatalf("remaining=%s", remaining)
	}
	if scout.ID != 0 {
		t.Fatalf("保存されている id=%d", scout.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("クールダウン中に INSERT した: %v", err)
	}
}

// TestCountByCompanySince_企業と期間で数える は、送信上限の判定（SOCAIAGENT-694）が
// 企業IDと開始時刻の両方で絞った件数を使うことを固定する。
func TestCountByCompanySince_企業と期間で数える(t *testing.T) {
	repo, mock := newScoutRepoTestDB(t)
	since := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `scouts` WHERE company_id = \\? AND created_at >= \\?").
		WithArgs(1, since).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))
	n, err := repo.CountByCompanySince(context.Background(), 1, since)
	if err != nil || n != 7 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
