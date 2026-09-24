package repositories_test

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/internal/models"
	"Backend/internal/repositories"
)

func newSchoolAppRepo(t *testing.T) (*repositories.SchoolCompanyApplicationRepository, sqlmock.Sqlmock) {
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
	return repositories.NewSchoolCompanyApplicationRepository(db), mock
}

// TestHasPending_FiltersByStatus は pending だけを数えることを検証する。
// status を条件から落とすと、却下済み申請でも重複扱いになり再申請できなくなる。
func TestHasPending_FiltersByStatus(t *testing.T) {
	repo, mock := newSchoolAppRepo(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `school_company_applications` WHERE school_id = \\? AND company_id = \\? AND status = \\?").
		WithArgs(uint(3), uint(7), models.SchoolCompanyApplicationPending).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	ok, err := repo.HasPending(3, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("want pending exists")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// TestListByCompany_ScopedByCompany は自社の申請だけを引くことを検証する（越境防止）。
func TestListByCompany_ScopedByCompany(t *testing.T) {
	repo, mock := newSchoolAppRepo(t)
	mock.ExpectQuery("SELECT \\* FROM `school_company_applications` WHERE company_id = \\?").
		WithArgs(uint(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "school_id", "company_id", "status"}).AddRow(1, 3, 7, "pending"))

	apps, err := repo.ListByCompany(7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 1 || apps[0].CompanyID != 7 {
		t.Errorf("apps = %+v", apps)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
