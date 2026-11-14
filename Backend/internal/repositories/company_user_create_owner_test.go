package repositories

import (
	"errors"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/domain/repository"
	"Backend/internal/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	mysqldriver "github.com/go-sql-driver/mysql"
)

// 企業とオーナーの同時作成(CreateOwnerWithCompany)のテスト。
// 担当者が企業に紐づかないまま保存されたり、片方だけ残ったりすると、
// 所有者のいない企業や、企業のいない担当者ができてしまう。

func newCompanyOwnerTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
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
	return db, mock
}

func newOwnerFixtures() (*models.Company, *models.CompanyUser) {
	return &models.Company{Name: "株式会社テスト", SourceType: "manual", DataStatus: "draft"},
		&models.CompanyUser{Email: "hr@example.com", Password: "hashed", Name: "担当", Role: models.CompanyUserRoleOwner}
}

func TestCreateOwnerWithCompany_LinksUserToCreatedCompany(t *testing.T) {
	db, mock := newCompanyOwnerTestDB(t)
	company, user := newOwnerFixtures()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `companies`").WillReturnResult(sqlmock.NewResult(77, 1))
	mock.ExpectExec("INSERT INTO `company_users`").WillReturnResult(sqlmock.NewResult(5, 1))
	mock.ExpectCommit()

	if err := NewCompanyUserRepository(db).CreateOwnerWithCompany(company, user); err != nil {
		t.Fatalf("CreateOwnerWithCompany: %v", err)
	}
	if company.ID != 77 {
		t.Fatalf("company.ID = %d, want 77", company.ID)
	}
	if user.CompanyID != 77 {
		t.Fatalf("user.CompanyID = %d, want 77(作った企業の ID を担当者に入れること)", user.CompanyID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}
}

func TestCreateOwnerWithCompany_UniqueViolationIsEmailTaken(t *testing.T) {
	db, mock := newCompanyOwnerTestDB(t)
	company, user := newOwnerFixtures()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `companies`").WillReturnResult(sqlmock.NewResult(77, 1))
	mock.ExpectExec("INSERT INTO `company_users`").
		WillReturnError(&mysqldriver.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectRollback() // 企業も巻き戻す

	err := NewCompanyUserRepository(db).CreateOwnerWithCompany(company, user)
	if !errors.Is(err, repository.ErrCompanyUserEmailTaken) {
		t.Fatalf("err = %v, want ErrCompanyUserEmailTaken", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}
}

func TestCreateOwnerWithCompany_OtherUserInsertErrorIsReturnedAsIs(t *testing.T) {
	db, mock := newCompanyOwnerTestDB(t)
	company, user := newOwnerFixtures()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `companies`").WillReturnResult(sqlmock.NewResult(77, 1))
	mock.ExpectExec("INSERT INTO `company_users`").
		WillReturnError(&mysqldriver.MySQLError{Number: 1213, Message: "Deadlock"})
	mock.ExpectRollback()

	err := NewCompanyUserRepository(db).CreateOwnerWithCompany(company, user)
	if err == nil || errors.Is(err, repository.ErrCompanyUserEmailTaken) {
		t.Fatalf("err = %v, 重複メール以外のエラーとして返すこと", err)
	}
}

func TestCreateOwnerWithCompany_CompanyInsertFailureCreatesNoUser(t *testing.T) {
	db, mock := newCompanyOwnerTestDB(t)
	company, user := newOwnerFixtures()
	boom := errors.New("db down")

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `companies`").WillReturnError(boom)
	mock.ExpectRollback() // company_users への INSERT は期待しない(来たら sqlmock が失敗させる)

	err := NewCompanyUserRepository(db).CreateOwnerWithCompany(company, user)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want db down", err)
	}
	if user.CompanyID != 0 {
		t.Fatalf("企業の作成に失敗したのに user.CompanyID = %d", user.CompanyID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}
}
