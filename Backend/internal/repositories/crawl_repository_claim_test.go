package repositories_test

import (
	"context"
	"testing"
	"time"

	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestCrawlRepository_ClaimSource(t *testing.T) {
	tests := []struct {
		name      string
		onlyIfDue bool
		affected  int64
		want      bool
	}{
		{"手動: 確保できた", false, 1, true},
		{"手動: 他が確保済み", false, 0, false},
		{"定期: 確保できた", true, 1, true},
		{"定期: 期限前または確保済み", true, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newTestDB(t)
			now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
			until := now.Add(2 * time.Hour)
			mock.ExpectBegin()
			if tt.onlyIfDue {
				mock.ExpectExec("UPDATE `crawl_sources` SET `next_run_at`=\\?.*WHERE id = \\? AND next_run_at <= \\?").
					WithArgs(until, sqlmock.AnyArg(), uint(7), now).
					WillReturnResult(sqlmock.NewResult(0, tt.affected))
			} else {
				mock.ExpectExec("UPDATE `crawl_sources` SET `next_run_at`=\\?.*WHERE id = \\? AND \\(next_run_at IS NULL OR next_run_at <= \\? OR next_run_at > \\?\\)").
					WithArgs(until, sqlmock.AnyArg(), uint(7), now, until).
					WillReturnResult(sqlmock.NewResult(0, tt.affected))
			}
			mock.ExpectCommit()
			got, err := repositories.NewCrawlRepository(db).ClaimSource(context.Background(), 7, now, 2*time.Hour, tt.onlyIfDue)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
