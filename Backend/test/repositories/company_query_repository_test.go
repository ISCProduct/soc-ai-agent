package repositories_test

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/internal/repositories"
)

func newCompanyQueryRepoTestDB(t *testing.T) (*repositories.CompanyQueryRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock作成失敗: %v", err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm open失敗: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return repositories.NewCompanyQueryRepository(db), mock
}

func TestGetCompaniesFiltered_DefaultOrderIsDeterministic(t *testing.T) {
	repo, mock := newCompanyQueryRepoTestDB(t)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `companies` WHERE is_active = \\?").
		WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery("SELECT \\* FROM `companies` WHERE is_active = \\? ORDER BY updated_at DESC, id ASC LIMIT \\?").
		WithArgs(true, 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "is_active"}).
			AddRow(2, "B社", true).
			AddRow(1, "A社", true))

	companies, total, err := repo.GetCompaniesFiltered(10, 0, "", "", "")
	if err != nil {
		t.Fatalf("GetCompaniesFiltered returned error: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(companies) != 2 {
		t.Fatalf("expected 2 companies, got %d", len(companies))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

func TestGetCompaniesFiltered_NameSearchOrdersByNameAsc(t *testing.T) {
	repo, mock := newCompanyQueryRepoTestDB(t)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `companies` WHERE is_active = \\? AND name LIKE \\?").
		WithArgs(true, "%テック%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery("SELECT \\* FROM `companies` WHERE is_active = \\? AND name LIKE \\? ORDER BY name ASC LIMIT \\?").
		WithArgs(true, "%テック%", 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "is_active"}).
			AddRow(1, "テック株式会社", true))

	companies, total, err := repo.GetCompaniesFiltered(10, 0, "", "テック", "")
	if err != nil {
		t.Fatalf("GetCompaniesFiltered returned error: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(companies) != 1 {
		t.Fatalf("expected 1 company, got %d", len(companies))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}
