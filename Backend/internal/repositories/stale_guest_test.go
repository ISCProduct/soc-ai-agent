package repositories

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"context"
	"errors"
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

// 一覧のあとで本登録・利用されたゲストを消さないよう、退会のトランザクションの中で条件を確かめ直す（はらさんのレビュー）。
// 外れたら ErrNotStaleGuest で巻き戻し、退会の記録もユーザーの更新も書かない。
func TestUserWithdrawalRepository_WithdrawStaleGuest_RechecksInTransaction(t *testing.T) {
	cutoff := time.Date(2026, 11, 15, 3, 0, 0, 0, time.UTC)
	now := cutoff.Add(7 * 24 * time.Hour)
	old := cutoff.Add(-time.Hour)
	errPassed := errors.New("確かめ直しを通過して退会の本体に進んだ")
	countSQL := regexp.QuoteMeta("SELECT count(*) FROM `user_refresh_tokens` WHERE user_id = ? AND created_at >= ?")

	tests := []struct {
		name      string
		isGuest   bool
		isAdmin   bool
		createdAt time.Time
		recent    *int // nil ならトークンを数える前に外れる
		wantErr   error
	}{
		{"本登録に進んだ（ゲストでない）", false, false, old, nil, repository.ErrNotStaleGuest},
		{"管理者", true, true, old, nil, repository.ErrNotStaleGuest},
		{"作成が cutoff ちょうど", true, false, cutoff, nil, repository.ErrNotStaleGuest},
		{"一覧のあとでログインした（cutoff 以降のトークンあり）", true, false, old, ptr(1), repository.ErrNotStaleGuest},
		{"まだ対象（退会の本体に進む）", true, false, old, ptr(0), errPassed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newWithdrawalTestDB(t)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT \\* FROM `users`").
				WillReturnRows(sqlmock.NewRows([]string{"id", "is_guest", "is_admin", "created_at", "withdrawn_at"}).
					AddRow(9, tt.isGuest, tt.isAdmin, tt.createdAt, nil))
			if tt.recent != nil {
				mock.ExpectQuery(countSQL).WithArgs(9, cutoff).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(*tt.recent))
			}
			if tt.wantErr == errPassed {
				mock.ExpectQuery(".").WillReturnError(errPassed)
			}
			mock.ExpectRollback()

			err := NewUserWithdrawalRepository(db).WithdrawStaleGuest(context.Background(), 9, cutoff, now, func(*models.User, []string) models.WithdrawnUser {
				t.Fatal("確かめ直しの前に退会の記録を作った")
				return models.WithdrawnUser{}
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("SQL が想定と違う: %v", err)
			}
		})
	}
}

func ptr(n int) *int { return &n }
