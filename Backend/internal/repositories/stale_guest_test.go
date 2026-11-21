package repositories

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// 掃除の対象は「ゲスト・未退会・管理者でない・作成から7日超・その後リフレッシュトークンが発行されていない」に限る (SOCAIAGENT-689)。
// 通常ユーザーを消さないよう、条件を SQL ごと固定する。
func TestUserWithdrawalRepository_ListStaleGuestIDs(t *testing.T) {
	db, mock := newWithdrawalTestDB(t)
	cutoff := time.Date(2026, 11, 15, 3, 0, 0, 0, time.UTC)
	want := "SELECT `id` FROM `users` WHERE (is_guest = ? AND is_admin = ? AND withdrawn_at IS NULL AND created_at < ?) " +
		"AND (NOT EXISTS (SELECT 1 FROM user_refresh_tokens t WHERE t.user_id = users.id AND t.created_at >= ?)) ORDER BY id LIMIT ?"
	mock.ExpectQuery("^"+regexp.QuoteMeta(want)+"$").
		WithArgs(true, false, cutoff, cutoff, 100).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3).AddRow(8))

	ids, err := NewUserWithdrawalRepository(db).ListStaleGuestIDs(context.Background(), cutoff, 100)
	if err != nil {
		t.Fatalf("ListStaleGuestIDs: %v", err)
	}
	if len(ids) != 2 || ids[0] != 3 || ids[1] != 8 {
		t.Fatalf("ids = %v", ids)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL が想定と違う: %v", err)
	}
	if _, err := NewUserWithdrawalRepository(db).ListStaleGuestIDs(context.Background(), cutoff, 0); err == nil {
		t.Fatal("limit 0 は弾く（GORM は負数を上限なしと解釈する）")
	}
}
