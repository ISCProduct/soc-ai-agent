package repositories

import (
	"testing"

	"Backend/internal/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
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
