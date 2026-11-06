package repositories

import (
	"database/sql/driver"
	"errors"
	"testing"

	"Backend/internal/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newUserScoreRecalculationRepoTest(t *testing.T) (*UserScoreRecalculationRepository, sqlmock.Sqlmock) {
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
	return NewUserScoreRecalculationRepository(db), mock
}

func TestUserScoreRecalculationRepository_CreateMapsActiveSlotConflict(t *testing.T) {
	tests := []struct {
		name    string
		dbErr   error
		wantErr error
	}{
		{name: "一意制約違反は実行中ジョブありとして返す", dbErr: &mysqldriver.MySQLError{Number: 1062}, wantErr: ErrUserScoreRecalculationRunning},
		{name: "それ以外のエラーはそのまま返す", dbErr: errors.New("connection lost"), wantErr: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newUserScoreRecalculationRepoTest(t)
			mock.ExpectBegin()
			mock.ExpectExec("INSERT INTO `user_score_recalculation_jobs`").WillReturnError(tt.dbErr)
			mock.ExpectRollback()

			err := repo.Create(&models.UserScoreRecalculationJob{Status: models.UserScoreRecalculationJobRunning})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err == nil || errors.Is(err, ErrUserScoreRecalculationRunning) {
				t.Fatalf("err = %v, want original error", err)
			}
		})
	}
}

func TestUserScoreRecalculationRepository_CancelOnlyPausedJobWithoutLiveLease(t *testing.T) {
	tests := []struct {
		name         string
		rowsAffected int64
		wantErr      error
	}{
		{name: "一時停止中でバッチが無ければ中止できる", rowsAffected: 1},
		{name: "条件に合う行が無ければ状態変更エラー", rowsAffected: 0, wantErr: ErrUserScoreRecalculationStatusChanged},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newUserScoreRecalculationRepoTest(t)
			mock.ExpectBegin()
			mock.ExpectExec(`UPDATE .user_score_recalculation_jobs. SET .* WHERE id = \? AND status = \? AND \(batch_lease_until IS NULL OR batch_lease_until < \?\)`).
				WithArgs(models.UserScoreRecalculationJobCancelled, sqlmock.AnyArg(), uint(12), models.UserScoreRecalculationJobPaused, sqlmock.AnyArg()).
				WillReturnResult(sqlmock.NewResult(0, tt.rowsAffected))
			mock.ExpectCommit()

			if err := repo.Cancel(12); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("expectations not met: %v", err)
			}
		})
	}
}

func TestUserScoreRecalculationRepository_AdvanceRecordsFailureKindOnlyOnFailure(t *testing.T) {
	tests := []struct {
		name    string
		failure string
	}{
		{name: "成功では失敗件数と分類を更新しない", failure: ""},
		{name: "失敗では分類を保存する", failure: models.UserScoreRecalculationFailureSave},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newUserScoreRecalculationRepoTest(t)
			args := []driver.Value{uint(34), sqlmock.AnyArg(), uint(12), "token"}
			pattern := "SET `cursor_score_id`=\\?,`processed_sessions`=processed_sessions \\+ 1,`updated_at`=\\?"
			if tt.failure != "" {
				pattern = "SET `cursor_score_id`=\\?,`failed_sessions`=failed_sessions \\+ 1,`last_error`=\\?,`processed_sessions`=processed_sessions \\+ 1,`updated_at`=\\?"
				args = []driver.Value{uint(34), tt.failure, sqlmock.AnyArg(), uint(12), "token"}
			}
			mock.ExpectBegin()
			mock.ExpectExec(pattern).WithArgs(args...).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			if err := repo.Advance(12, 34, "token", tt.failure); err != nil {
				t.Fatalf("Advance returned error: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("expectations not met: %v", err)
			}
		})
	}
}

func TestUserScoreRecalculationRepository_PauseOnlyUpdatesRunningJob(t *testing.T) {
	repo, mock := newUserScoreRecalculationRepoTest(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user_score_recalculation_jobs` SET .* WHERE id = \\? AND status = \\?").
		WithArgs(models.UserScoreRecalculationJobPaused, sqlmock.AnyArg(), uint(12), models.UserScoreRecalculationJobRunning).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.UpdateStatus(12, models.UserScoreRecalculationJobPaused); err != nil {
		t.Fatalf("UpdateStatus returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}
