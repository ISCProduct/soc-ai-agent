package repositories

import (
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newLastActivityRepo(t *testing.T) (*UserRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return NewUserRepository(db), mock
}

// 最終活動の定義（#1573）: 生徒自身のチャット発言・応募更新・スコア更新の最大値。
// どれか1つでも抜けると、その経路だけで活動している生徒が「未活動」に出る。
func TestFindLastActivityByUsers_UsesThreeSources(t *testing.T) {
	repo, mock := newLastActivityRepo(t)
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`(?s)FROM chat_messages\s+WHERE user_id IN \(\?,\?\) AND role = 'user'.*`+
		`FROM user_application_statuses\s+WHERE user_id IN \(\?,\?\).*`+
		`FROM user_weight_scores\s+WHERE user_id IN \(\?,\?\)`).
		WithArgs(1, 2, 1, 2, 1, 2).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "last_at"}).AddRow(1, at))

	got, err := repo.FindLastActivityByUsers([]uint{1, 2})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.Len(t, got, 1, "記録の無い生徒は map に入れない")
	require.True(t, got[1].Equal(at))
}

// 空スライスではDBに触らない。IN () は構文エラーになるため。
func TestFindLastActivityByUsers_EmptyInput(t *testing.T) {
	repo, mock := newLastActivityRepo(t)

	got, err := repo.FindLastActivityByUsers(nil)
	require.NoError(t, err)
	require.Empty(t, got)
	require.NoError(t, mock.ExpectationsWereMet())
}
