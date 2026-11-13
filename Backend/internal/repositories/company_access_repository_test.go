package repositories

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newCompanyAccessTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("sqlmock: %v", err)
		}
		_ = sqlDB.Close()
	})
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}
	return db, mock
}

func TestCompanyAccessRepository_NilIsFailClosed(t *testing.T) {
	var nilRepo *CompanyAccessRepository
	for name, repo := range map[string]*CompanyAccessRepository{"nil リポジトリ": nilRepo, "db が nil": NewCompanyAccessRepository(nil)} {
		if ok, err := repo.IsAdmin(1); err != nil || ok {
			t.Fatalf("%s: IsAdmin ok=%v err=%v want false,nil", name, ok, err)
		}
		if ok, err := repo.OwnsCompany(1, 10); err != nil || ok {
			t.Fatalf("%s: OwnsCompany ok=%v err=%v want false,nil", name, ok, err)
		}
	}
}

func TestCompanyAccessRepository_ZeroCompanyIsFalseWithoutQuery(t *testing.T) {
	db, _ := newCompanyAccessTestDB(t) // クエリが来たら sqlmock が失敗させる
	ok, err := NewCompanyAccessRepository(db).OwnsCompany(1, 0)
	if err != nil || ok {
		t.Fatalf("companyID=0: got ok=%v err=%v", ok, err)
	}
}

func TestCompanyAccessRepository_IsAdmin(t *testing.T) {
	db, mock := newCompanyAccessTestDB(t)
	mock.ExpectQuery("SELECT .*is_admin.* FROM `users` WHERE id = \\?").
		WithArgs(uint(7)).
		WillReturnRows(sqlmock.NewRows([]string{"is_admin"}).AddRow(true))
	ok, err := NewCompanyAccessRepository(db).IsAdmin(7)
	if err != nil || !ok {
		t.Fatalf("got ok=%v err=%v want true", ok, err)
	}
}

func TestCompanyAccessRepository_OwnsCompany(t *testing.T) {
	db, mock := newCompanyAccessTestDB(t)
	repo := NewCompanyAccessRepository(db)
	query := "SELECT count\\(\\*\\) FROM `company_ownerships` WHERE user_id = \\? AND company_id = \\?"

	mock.ExpectQuery(query).WithArgs(uint(1), uint(10)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(1))
	if ok, err := repo.OwnsCompany(1, 10); err != nil || !ok {
		t.Fatalf("got ok=%v err=%v want true", ok, err)
	}

	mock.ExpectQuery(query).WithArgs(uint(1), uint(99)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(0))
	if ok, err := repo.OwnsCompany(1, 99); err != nil || ok {
		t.Fatalf("got ok=%v err=%v want false", ok, err)
	}
}
