package repositories

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newCollectiveInsightRepo(t *testing.T) (*CollectiveInsightRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return NewCollectiveInsightRepository(db), mock
}

var similarUserPassedColumns = []string{"company_id", "company_name", "pass_count", "avg_passer_scores", "collective_score"}

// 類似ユーザーの通過企業は、通過/応募のログだけを数え、上位20社に絞る。
func TestCollectiveInsightRepository_GetPassedCompaniesBySimilarUsers_除外なし(t *testing.T) {
	repo, mock := newCollectiveInsightRepo(t)
	mock.ExpectQuery(`cil\.action_type IN \('passed', 'applied'\)\s+GROUP BY cil\.company_id, c\.name\s+ORDER BY pass_count DESC\s+LIMIT 20`).
		WithArgs(2, "h1", "h2").
		WillReturnRows(sqlmock.NewRows(similarUserPassedColumns).AddRow(7, "A社", 2, "", 1.0))

	rows, err := repo.GetPassedCompaniesBySimilarUsers([]string{"h1", "h2"}, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, uint(7), rows[0].CompanyID)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 除外する企業IDを渡したときだけ、NOT IN で外す。
func TestCollectiveInsightRepository_GetPassedCompaniesBySimilarUsers_除外あり(t *testing.T) {
	repo, mock := newCollectiveInsightRepo(t)
	mock.ExpectQuery(`cil\.action_type IN \('passed', 'applied'\)\s+AND cil\.company_id NOT IN \(\?,\?\)\s+GROUP BY`).
		WithArgs(1, "h1", uint(3), uint(4)).
		WillReturnRows(sqlmock.NewRows(similarUserPassedColumns))

	rows, err := repo.GetPassedCompaniesBySimilarUsers([]string{"h1"}, []uint{3, 4})
	require.NoError(t, err)
	require.Empty(t, rows)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 類似ユーザーが0人なら、DBに問い合わせない。
func TestCollectiveInsightRepository_GetPassedCompaniesBySimilarUsers_ユーザー無しは問い合わせない(t *testing.T) {
	repo, mock := newCollectiveInsightRepo(t)
	rows, err := repo.GetPassedCompaniesBySimilarUsers(nil, []uint{1})
	require.NoError(t, err)
	require.Nil(t, rows)
	require.NoError(t, mock.ExpectationsWereMet())
}
