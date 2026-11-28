package repositories

import (
	"context"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// 再設定トークンは新しい列（ハッシュ）だけで引く。旧列（平文）は読まない (SOCAIAGENT-693)。
func TestUserRepository_GetUserByPasswordResetTokenHash(t *testing.T) {
	db, mock := newWithdrawalTestDB(t)
	want := "SELECT * FROM `users` WHERE password_reset_token_hash = ? ORDER BY `users`.`id` LIMIT ?"
	mock.ExpectQuery("^"+regexp.QuoteMeta(want)+"$").
		WithArgs("abc123", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(4))

	u, err := NewUserRepository(db).GetUserByPasswordResetTokenHash(context.Background(), "abc123")
	if err != nil || u == nil || u.ID != 4 {
		t.Fatalf("u = %v err = %v", u, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL が想定と違う: %v", err)
	}
}
