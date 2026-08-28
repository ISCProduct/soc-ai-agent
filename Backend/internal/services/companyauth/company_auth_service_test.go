package companyauth

import (
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newCompanyAuthTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
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

func newTestService(t *testing.T, db *gorm.DB) *CompanyUserService {
	t.Helper()
	return NewCompanyUserService(
		db,
		repositories.NewCompanyUserRepository(db),
		repositories.NewCompanyUserRefreshTokenRepository(db),
		nil,
		"test-company-secret",
	)
}

func TestInvite_RejectsUnverifiedCompany(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)

	mock.ExpectQuery("SELECT \\* FROM `companies`").
		WithArgs(uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "is_verified"}).AddRow(1, false))

	_, err := svc.Invite(1, InviteRequest{Email: "hr@example.com", Name: "担当者"})
	if err != ErrCompanyNotVerified {
		t.Fatalf("expected ErrCompanyNotVerified, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestLogin_InvalidPassword(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)

	hashed, err := bcrypt.GenerateFromPassword([]byte("correct-pass"), bcryptCost)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs("hr@example.com", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password", "name", "role"}).
			AddRow(1, 10, "hr@example.com", string(hashed), "担当", models.CompanyUserRoleOwner))

	_, err = svc.Login(LoginRequest{Email: "hr@example.com", Password: "wrong"})
	if err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestEnsureCompanyAccess_ForbiddenForOtherCompany(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)

	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs(uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password", "name", "role"}).
			AddRow(1, 10, "hr@example.com", "hash", "担当", models.CompanyUserRoleOwner))

	err := svc.EnsureCompanyAccess(1, 99)
	if err == nil || err.Error() != "forbidden" {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAcceptInvite_ExpiredToken(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = func() time.Time { return time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC) }
	expired := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs("tok", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password", "invite_expires_at"}).
			AddRow(1, 10, "hr@example.com", "", expired))

	_, err := svc.AcceptInvite(AcceptInviteRequest{Token: "tok", Password: "password1"})
	if err != ErrInviteExpired {
		t.Fatalf("expected ErrInviteExpired, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
