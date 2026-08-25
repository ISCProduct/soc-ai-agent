package shared

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newOwnerTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}
	return db, mock
}

func TestUserIsAdmin_NilDBIsFalse(t *testing.T) {
	ok, err := UserIsAdmin(nil, 1)
	if err != nil || ok {
		t.Fatalf("got ok=%v err=%v want false,nil", ok, err)
	}
}

func TestUserOwnsCompany_NilDBOrZeroCompanyIsFalse(t *testing.T) {
	ok, err := UserOwnsCompany(nil, 1, 10)
	if err != nil || ok {
		t.Fatalf("nil db: got ok=%v err=%v", ok, err)
	}
	db, _ := newOwnerTestDB(t)
	ok, err = UserOwnsCompany(db, 1, 0)
	if err != nil || ok {
		t.Fatalf("companyID=0: got ok=%v err=%v", ok, err)
	}
}

func TestUserIsAdmin_True(t *testing.T) {
	db, mock := newOwnerTestDB(t)
	mock.ExpectQuery("SELECT .*is_admin.* FROM `users` WHERE id = \\?").
		WithArgs(uint(7)).
		WillReturnRows(sqlmock.NewRows([]string{"is_admin"}).AddRow(true))
	ok, err := UserIsAdmin(db, 7)
	if err != nil || !ok {
		t.Fatalf("got ok=%v err=%v want true", ok, err)
	}
}

func TestUserOwnsCompany_Count(t *testing.T) {
	db, mock := newOwnerTestDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `company_ownerships` WHERE user_id = \\? AND company_id = \\?").
		WithArgs(uint(1), uint(10)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(1))
	ok, err := UserOwnsCompany(db, 1, 10)
	if err != nil || !ok {
		t.Fatalf("got ok=%v err=%v want true", ok, err)
	}

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `company_ownerships` WHERE user_id = \\? AND company_id = \\?").
		WithArgs(uint(1), uint(99)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(0))
	ok, err = UserOwnsCompany(db, 1, 99)
	if err != nil || ok {
		t.Fatalf("got ok=%v err=%v want false", ok, err)
	}
}
