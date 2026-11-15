package repositories

import (
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 通過率（passed / applied）が 100% を超えないよう、1人1企業1種別の記録済みかどうかを引く。
// 匿名ユーザー・企業・種別の3つで絞らないと、別の人や別の種別の行を「記録済み」と誤判定する。
func TestCollectiveInsightRepository_HasAction(t *testing.T) {
	query := regexp.QuoteMeta("SELECT count(*) FROM `collective_insight_logs` WHERE anonymous_user_id = ? AND company_id = ? AND action_type = ? LIMIT ?")
	for _, tc := range []struct {
		name  string
		count int
		want  bool
	}{
		{"記録済み", 1, true},
		{"未記録", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newMockDB(t)
			mock.ExpectQuery(query).WithArgs("anon-1", uint(10), "passed", 1).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tc.count))

			got, err := NewCollectiveInsightRepository(db).HasAction("anon-1", 10, "passed")

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
