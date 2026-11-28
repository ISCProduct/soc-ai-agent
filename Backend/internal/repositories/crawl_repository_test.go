package repositories

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 定期取得の実行権（GET_LOCK）。取れたタスクだけが実行し、必ず解放する（SOCAIAGENT-723）。
func TestCrawlRepository_WithRunLock(t *testing.T) {
	tests := []struct {
		name       string
		lockResult any
		wantCalled bool
	}{
		{"取得できれば実行して解放する", int64(1), true},
		{"他のタスクが実行中なら実行しない", int64(0), false},
		{"NULL（エラー）なら実行しない", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
				&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)

			mock.ExpectQuery("SELECT GET_LOCK").
				WithArgs("crawl_run_due_sources").
				WillReturnRows(sqlmock.NewRows([]string{"GET_LOCK"}).AddRow(tt.lockResult))
			if tt.wantCalled {
				mock.ExpectExec("SELECT RELEASE_LOCK").
					WithArgs("crawl_run_due_sources").
					WillReturnResult(sqlmock.NewResult(0, 0))
			}

			called := false
			acquired, err := NewCrawlRepository(db).WithRunLock(context.Background(), func() { called = true })
			require.NoError(t, err)
			require.Equal(t, tt.wantCalled, acquired)
			require.Equal(t, tt.wantCalled, called)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
