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

func TestRefreshSession_EmptyToken(t *testing.T) {
	db, _ := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)

	_, err := svc.RefreshSession("")
	if err != ErrInvalidRefreshToken {
		t.Fatalf("expected ErrInvalidRefreshToken, got %v", err)
	}
}

func TestRefreshSession_UnknownToken(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)

	mock.ExpectQuery("SELECT \\* FROM `company_user_refresh_tokens`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err := svc.RefreshSession("unknown-token")
	if err != ErrInvalidRefreshToken {
		t.Fatalf("expected ErrInvalidRefreshToken, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRefreshSession_ExpiredToken(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = func() time.Time { return time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC) }
	expired := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT \\* FROM `company_user_refresh_tokens`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_user_id", "token_hash", "expires_at"}).
			AddRow(1, 1, "hash", expired))

	_, err := svc.RefreshSession("expired-token")
	if err != ErrInvalidRefreshToken {
		t.Fatalf("expected ErrInvalidRefreshToken, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestLogoutSession_EmptyTokenIsNoop(t *testing.T) {
	db, _ := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)

	if err := svc.LogoutSession(""); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestLogoutSession_UnknownTokenIsNoop(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)

	mock.ExpectQuery("SELECT \\* FROM `company_user_refresh_tokens`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if err := svc.LogoutSession("unknown-token"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// ログアウト直後にRefreshSessionのローテーション猶予期間(60秒)が誤って適用され、
// 同じトークンでリフレッシュが成功してしまう回帰を防ぐ。
func TestRefreshSession_RejectsTokenImmediatelyAfterLogout(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	mock.ExpectQuery("SELECT \\* FROM `company_user_refresh_tokens`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `company_user_refresh_tokens`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := svc.LogoutSession("logged-out-token"); err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	// ログアウト直後(猶予期間内)に同じトークンでリフレッシュしても、
	// レコードごと削除済みなのでFindByHashはnilを返し、必ず拒否される。
	mock.ExpectQuery("SELECT \\* FROM `company_user_refresh_tokens`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err := svc.RefreshSession("logged-out-token")
	if err != ErrInvalidRefreshToken {
		t.Fatalf("expected ErrInvalidRefreshToken after logout, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
