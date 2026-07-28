package repositories_test

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/internal/repositories"
)

func newCompanyRepoTestDB(t *testing.T) (*repositories.CompanyRepository, sqlmock.Sqlmock) {
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
	return repositories.NewCompanyRepository(db), mock
}

func TestListActiveFiltered_ByNameAndStatus(t *testing.T) {
	repo, mock := newCompanyRepoTestDB(t)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `companies` WHERE is_active = \\? AND name LIKE \\?").
		WithArgs(true, "%ソニー%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery("SELECT \\* FROM `companies` WHERE is_active = \\? AND name LIKE \\? ORDER BY id desc LIMIT \\?").
		WithArgs(true, "%ソニー%", 50).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "data_status", "is_active"}).
			AddRow(2, "ソニー損保", "draft", true).
			AddRow(1, "ソニー株式会社", "published", true))

	all, total, err := repo.ListActiveFiltered(50, 0, "ソニー", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 || len(all) != 2 {
		t.Fatalf("want total=2 len=2, got total=%d len=%d", total, len(all))
	}

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `companies` WHERE is_active = \\? AND name LIKE \\? AND data_status = \\?").
		WithArgs(true, "%ソニー%", "published").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT \\* FROM `companies` WHERE is_active = \\? AND name LIKE \\? AND data_status = \\? ORDER BY id desc LIMIT \\?").
		WithArgs(true, "%ソニー%", "published", 50).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "data_status", "is_active"}).
			AddRow(1, "ソニー株式会社", "published", true))

	published, total, err := repo.ListActiveFiltered(50, 0, "ソニー", "published")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 || len(published) != 1 || published[0].Name != "ソニー株式会社" {
		t.Fatalf("unexpected published result: total=%d companies=%v", total, published)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
