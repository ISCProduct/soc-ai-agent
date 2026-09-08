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

	// 列指定更新であること。Save(全カラム更新)だと、並行する無効化操作を
	// 巻き戻して disabled_at を消してしまう（レビュー指摘 Blocker）。
	var savedHash string
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `company_users` SET `password_reset_expires_at`=\\?,`password_reset_token_hash`=\\?,`updated_at`=\\? WHERE id = \\?").
		WillReturnResult(sqlmock.NewResult(0, 1))
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

// --- RefreshSession --------------------------------------------------------

// TestRefreshSession_DisabledAccountRejected は無効化の迂回を防げていることを検証する。
//
// SetDisabled は既存のリフレッシュトークンを失効させるが、rotationGracePeriod(60秒)の
// 間は失効済みトークンでもローテーションが通る。ここで無効化を見ていないと、
// 無効化直後にリフレッシュした利用者へ新しいJWTと未失効のリフレッシュトークンが渡り、
// 以降ずっとアクセスを維持できてしまう。
func TestRefreshSession_DisabledAccountRejected(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = fixedNow
	future := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	// 猶予期間内に失効したトークン（無効化と同時に revoke された直後を再現）。
	revoked := fixedNow().Add(-10 * time.Second)
	disabled := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT \\* FROM `company_user_refresh_tokens`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_user_id", "token_hash", "expires_at", "revoked_at"}).
			AddRow(5, 1, hashRefreshToken("plain"), future, revoked))
	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs(uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password", "disabled_at"}).
			AddRow(1, 10, "hr@example.com", "hashed", disabled))

	if _, err := svc.RefreshSession("plain"); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("無効化されたアカウントがリフレッシュできてしまう: err=%v", err)
	}
}

// 招待未受諾（パスワード未設定）でもリフレッシュを通さない。
func TestRefreshSession_PasswordNotSetRejected(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = fixedNow
	future := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT \\* FROM `company_user_refresh_tokens`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_user_id", "token_hash", "expires_at", "revoked_at"}).
			AddRow(5, 1, hashRefreshToken("plain"), future, nil))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `company_user_refresh_tokens`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs(uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password"}).
			AddRow(1, 10, "hr@example.com", ""))

	if _, err := svc.RefreshSession("plain"); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken, got %v", err)
	}
}

// --- レビュー指摘の再発防止 ------------------------------------------------

// TestResetPassword_ConsumesTokenAndRevokesSessions は、リセット成功時に
// トークンが使い捨てになり、既存セッションが全て切れることを検証する。
//
// 使い捨てにしないと TTL(1時間)の間はメールのリンクが何度でも使える。
// セッション失効を怠ると「パスワード変更で他端末を切る」保証が成立しない。
func TestResetPassword_ConsumesTokenAndRevokesSessions(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = fixedNow
	future := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs(hashToken("tok"), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password", "password_reset_expires_at"}).
			AddRow(1, 10, "hr@example.com", "old-hash", future))

	// パスワード設定とトークン破棄を1文で行い、無効化と競合しないこと。
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `company_users` SET .*password_reset.*WHERE id = \\? AND disabled_at IS NULL").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	// 既存リフレッシュトークンの全失効。
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `company_user_refresh_tokens` SET .*revoked_at").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	// 新しいリフレッシュトークンの発行。
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `company_user_refresh_tokens`").
		WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectCommit()

	if _, err := svc.ResetPassword(ResetPasswordRequest{Token: "tok", Password: "password1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("トークンの使い捨て、またはセッション失効が行われていない: %v", err)
	}
}

// bcrypt(約300ms)の最中に管理者が無効化した場合、0行更新で失敗すること。
// ここが通ると無効化したアカウントのパスワードだけ書き換わる。
func TestResetPassword_LostUpdateGuard(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = fixedNow
	future := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs(hashToken("tok"), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password", "password_reset_expires_at"}).
			AddRow(1, 10, "hr@example.com", "old-hash", future))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `company_users`").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if _, err := svc.ResetPassword(ResetPasswordRequest{Token: "tok", Password: "password1"}); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("expected ErrAccountDisabled, got %v", err)
	}
}

// セッション失効に失敗したら、新しい資格情報を返さないこと。
func TestResetPassword_FailsWhenRevokeFails(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = fixedNow
	future := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs(hashToken("tok"), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password", "password_reset_expires_at"}).
			AddRow(1, 10, "hr@example.com", "old-hash", future))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `company_users`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `company_user_refresh_tokens`").WillReturnError(errors.New("db down"))
	mock.ExpectRollback()

	resp, err := svc.ResetPassword(ResetPasswordRequest{Token: "tok", Password: "password1"})
	if err == nil {
		t.Fatal("失効に失敗したのに成功として返っている")
	}
	if resp != nil {
		t.Error("失効に失敗したのに新しい資格情報を返している")
	}
}

// SetDisabled が全カラム更新(Save)ではなく列指定であること。
// Save だと古いモデルの書き戻しで並行更新を巻き戻し、
// 無効化済みアカウントが復活しうる。
func TestSetDisabled_UsesColumnScopedUpdate(t *testing.T) {
	db, mock := newCompanyAuthTestDB(t)
	svc := newTestService(t, db)
	svc.now = fixedNow

	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WithArgs(uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email"}).AddRow(1, 10, "hr@example.com"))
	mock.ExpectBegin()
	// disabled_at と リセット列だけを更新し、email や password は触らないこと。
	mock.ExpectExec("UPDATE `company_users` SET `disabled_at`=\\?,`password_reset_expires_at`=\\?,`password_reset_token_hash`=\\?,`updated_at`=\\? WHERE id = \\?").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `company_user_refresh_tokens`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if _, err := svc.SetDisabled(10, 1, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("列指定更新になっていない（Save で全カラムを書き戻している可能性）: %v", err)
	}
}
