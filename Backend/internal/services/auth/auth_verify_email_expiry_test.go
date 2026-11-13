package auth

import (
	"testing"
	"time"

	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// メール認証トークンの有効期限(#330)の判定を、DB から読んだ行が mapper を通る形で確かめる。
//
// VerifyEmail は userRepo.GetUserByVerificationToken が返す models.User の
// EmailVerificationExpires を見る。mapper が期限をコピーしないと、DB に期限が入っていても
// entity では常に nil になり、期限切れの判定が働かない。スタブの userRepo では mapper を
// 通らないので、実の UserRepository と sqlmock で確かめる。

const verifyTokenQuery = "SELECT \\* FROM `users` WHERE email_verification_token = \\? ORDER BY `users`.`id` LIMIT \\?"

func TestVerifyEmail_ExpiredTokenIsRejected(t *testing.T) {
	db, mock := newAuthServiceTestDB(t)
	svc := &AuthService{userRepo: repositories.NewUserRepository(db)}

	expired := time.Now().Add(-time.Hour)
	mock.ExpectQuery(verifyTokenQuery).
		WithArgs("tok-expired", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "email_verification_token", "email_verification_expires"}).
			AddRow(1, "user@example.com", "tok-expired", expired))
	// 期限切れなので、認証済みにする UPDATE は来ない(来たら sqlmock が失敗させる)

	err := svc.VerifyEmail("tok-expired")
	if err == nil || err.Error() != "invalid or expired token" {
		t.Fatalf("err = %v, want invalid or expired token", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}
}

func TestVerifyEmail_UnexpiredTokenIsAccepted(t *testing.T) {
	db, mock := newAuthServiceTestDB(t)
	svc := &AuthService{userRepo: repositories.NewUserRepository(db)}

	valid := time.Now().Add(time.Hour)
	mock.ExpectQuery(verifyTokenQuery).
		WithArgs("tok-valid", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "email_verification_token", "email_verification_expires"}).
			AddRow(1, "user@example.com", "tok-valid", valid))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `users` SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := svc.VerifyEmail("tok-valid"); err != nil {
		t.Fatalf("期限内のトークンは通す: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}
}
