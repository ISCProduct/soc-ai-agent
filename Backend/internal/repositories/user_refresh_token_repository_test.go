package repositories

import (
	"context"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"
)

// 明示的な失効（ログアウト・全端末失効）は行を物理削除し、対象を ID / ユーザーで絞ることを固定する (SOCAIAGENT-688)。
func TestUserRefreshTokenRepository_ExplicitDelete(t *testing.T) {
	tests := []struct {
		name    string
		del     func(db *gorm.DB) error
		wantSQL string
		arg     uint
	}{
		{
			name:    "ログアウトは該当IDだけ削除",
			del:     func(db *gorm.DB) error { return NewUserRefreshTokenRepository(db).Delete(context.Background(), 7) },
			wantSQL: "DELETE FROM `user_refresh_tokens` WHERE `user_refresh_tokens`.`id` = ?",
			arg:     7,
		},
		{
			name: "全端末失効は該当ユーザーの行を失効済みも含めて削除",
			del: func(db *gorm.DB) error {
				return NewUserRefreshTokenRepository(db).DeleteAllByUser(context.Background(), 42)
			},
			wantSQL: "DELETE FROM `user_refresh_tokens` WHERE user_id = ?",
			arg:     42,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newExpiredDeleteTestDB(t)
			mock.ExpectBegin()
			mock.ExpectExec("^" + regexp.QuoteMeta(tt.wantSQL) + "$").WithArgs(tt.arg).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			if err := tt.del(db); err != nil {
				t.Fatalf("削除に失敗: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("SQL が想定と違う: %v", err)
			}
		})
	}
}
