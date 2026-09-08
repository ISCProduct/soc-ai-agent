package companyauth

import (
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
)

func fixedNow() time.Time { return time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC) }

// --- RequestPasswordReset -------------------------------------------------

// TestRequestPasswordReset_UnknownEmailIsSilentSuccess は、
// 存在しないメールでもエラーを返さないことを検証する。
// エラーの有無からアカウントの存在を推測されないため（#1196）。
func TestRequestPasswordReset_UnknownEmailIsSilentSuccess(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = fixedNow

	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs("nobody@example.com", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if err := svc.RequestPasswordReset(ForgotPasswordRequest{Email: "nobody@example.com"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	// UPDATE が発行されていないこと（＝トークンを作っていないこと）。
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// 招待未受諾（パスワード未設定）のアカウントにはリセットを発行しない。
// 受諾前にリセットを許すと、招待メールを見ていない第三者が乗っ取れる。
func TestRequestPasswordReset_InviteNotAcceptedIsSilentSuccess(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = fixedNow

	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs("hr@example.com", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password"}).
			AddRow(1, 10, "hr@example.com", ""))

	if err := svc.RequestPasswordReset(ForgotPasswordRequest{Email: "hr@example.com"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// 無効化済みアカウントにもリセットを発行しない（アクセス剥奪の迂回を防ぐ）。
func TestRequestPasswordReset_DisabledIsSilentSuccess(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = fixedNow
	disabled := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs("hr@example.com", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password", "disabled_at"}).
			AddRow(1, 10, "hr@example.com", "hashed", disabled))

	if err := svc.RequestPasswordReset(ForgotPasswordRequest{Email: "hr@example.com"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// 正常系: トークンのハッシュと期限が保存されること。平文は保存しない。
func TestRequestPasswordReset_StoresHashedToken(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = fixedNow

	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs("hr@example.com", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password"}).
			AddRow(1, 10, "hr@example.com", "hashed"))

	var savedHash string
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `company_users`").
		WillReturnResult(sqlmock.NewResult(0, 1)).
		WillDelayFor(0)
	mock.ExpectCommit()

	if err := svc.RequestPasswordReset(ForgotPasswordRequest{Email: "hr@example.com"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
	_ = savedHash
}

// --- ResetPassword --------------------------------------------------------

func TestResetPassword_ShortPasswordRejected(t *testing.T) {
	db, _ := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)

	if _, err := svc.ResetPassword(ResetPasswordRequest{Token: "tok", Password: "short"}); err == nil {
		t.Fatal("expected error for password shorter than 8 characters")
	}
}

func TestResetPassword_UnknownToken(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = fixedNow

	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs(hashToken("tok"), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if _, err := svc.ResetPassword(ResetPasswordRequest{Token: "tok", Password: "password1"}); !errors.Is(err, ErrResetTokenInvalid) {
		t.Fatalf("expected ErrResetTokenInvalid, got %v", err)
	}
}

// トークンは平文ではなくハッシュで照合される。
// 平文でDBを引いていたらこのテストが落ちる。
func TestResetPassword_LooksUpByHashNotPlaintext(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = fixedNow

	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs(hashToken("plain-token"), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, _ = svc.ResetPassword(ResetPasswordRequest{Token: "plain-token", Password: "password1"})
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("平文で検索している可能性がある: %v", err)
	}
}

func TestResetPassword_ExpiredToken(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = fixedNow
	expired := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs(hashToken("tok"), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password", "password_reset_expires_at"}).
			AddRow(1, 10, "hr@example.com", "hashed", expired))

	if _, err := svc.ResetPassword(ResetPasswordRequest{Token: "tok", Password: "password1"}); !errors.Is(err, ErrResetTokenExpired) {
		t.Fatalf("expected ErrResetTokenExpired, got %v", err)
	}
}

// 期限カラムが NULL のレコードは無効扱いにする（期限なしトークンを作らない）。
func TestResetPassword_NullExpiryTreatedAsExpired(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = fixedNow

	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs(hashToken("tok"), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password", "password_reset_expires_at"}).
			AddRow(1, 10, "hr@example.com", "hashed", nil))

	if _, err := svc.ResetPassword(ResetPasswordRequest{Token: "tok", Password: "password1"}); !errors.Is(err, ErrResetTokenExpired) {
		t.Fatalf("expected ErrResetTokenExpired, got %v", err)
	}
}

func TestResetPassword_DisabledAccountRejected(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = fixedNow
	future := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	disabled := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs(hashToken("tok"), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password", "password_reset_expires_at", "disabled_at"}).
			AddRow(1, 10, "hr@example.com", "hashed", future, disabled))

	if _, err := svc.ResetPassword(ResetPasswordRequest{Token: "tok", Password: "password1"}); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("expected ErrAccountDisabled, got %v", err)
	}
}

// --- Login ----------------------------------------------------------------

// 無効化されたアカウントはログインできない。
// ただし資格情報が誤っている場合は無効化の事実を伝えない。
func TestLogin_DisabledAccountRejected(t *testing.T) {
	hashed, err := bcrypt.GenerateFromPassword([]byte("password1"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	disabled := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("正しいパスワードなら無効化を伝える", func(t *testing.T) {
		db, mock := newCompanyAuthTestDB(t)
		svc := newTestService(t, db)
		svc.now = fixedNow
		mock.ExpectQuery("SELECT \\* FROM `company_users`").
			WithArgs("hr@example.com", 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password", "disabled_at"}).
				AddRow(1, 10, "hr@example.com", string(hashed), disabled))

		if _, err := svc.Login(LoginRequest{Email: "hr@example.com", Password: "password1"}); !errors.Is(err, ErrAccountDisabled) {
			t.Fatalf("expected ErrAccountDisabled, got %v", err)
		}
	})

	t.Run("誤ったパスワードでは無効化の事実を伝えない", func(t *testing.T) {
		db, mock := newCompanyAuthTestDB(t)
		svc := newTestService(t, db)
		svc.now = fixedNow
		mock.ExpectQuery("SELECT \\* FROM `company_users`").
			WithArgs("hr@example.com", 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password", "disabled_at"}).
				AddRow(1, 10, "hr@example.com", string(hashed), disabled))

		if _, err := svc.Login(LoginRequest{Email: "hr@example.com", Password: "wrong"}); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})
}

// --- SetDisabled ----------------------------------------------------------

// 他社の企業ユーザーは操作できない。
func TestSetDisabled_RejectsOtherCompany(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = fixedNow

	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs(uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email"}).
			AddRow(1, 10, "hr@example.com"))

	// companyID=99 は当該ユーザーの所属(10)と異なる。
	if _, err := svc.SetDisabled(99, 1, true); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}
