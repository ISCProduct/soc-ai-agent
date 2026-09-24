package repositories_test

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/internal/repositories"
)

func newSuppressionRepo(t *testing.T) (*repositories.SchoolJobSuppressionRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return repositories.NewSchoolJobSuppressionRepository(db), mock
}

func TestSuppressionListBySchool(t *testing.T) {
	repo, mock := newSuppressionRepo(t)
	mock.ExpectQuery("SELECT \\* FROM `school_job_suppressions` WHERE school_id = \\?").
		WithArgs(uint(3)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "school_id", "job_position_id"}).AddRow(1, 3, 55))
	list, err := repo.ListBySchool(3)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(list) != 1 || list[0].JobPositionID != 55 {
		t.Errorf("list = %+v", list)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// TestSuppressionDelete はscope（school_id AND job_position_id）で消すことを検証する。
func TestSuppressionDelete(t *testing.T) {
	repo, mock := newSuppressionRepo(t)
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `school_job_suppressions` WHERE school_id = \\? AND job_position_id = \\?").
		WithArgs(uint(3), uint(55)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repo.Delete(3, 55); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
