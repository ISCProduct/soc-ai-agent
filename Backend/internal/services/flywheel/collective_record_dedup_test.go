package flywheel

import (
	"regexp"
	"testing"

	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newDedupTestService(t *testing.T) (*CollectiveInsightService, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return NewCollectiveInsightService(repositories.NewCollectiveInsightRepository(db), repositories.NewUserWeightScoreRepository(db)), mock
}

var (
	consentQuery   = regexp.QuoteMeta("SELECT `allow_collective_insight` FROM `users`")
	hasActionQuery = regexp.QuoteMeta("SELECT count(*) FROM `collective_insight_logs` WHERE anonymous_user_id = ? AND company_id = ? AND action_type = ? LIMIT ?")
)

// 書類通過・内定・承諾はどれも passed に写るが、同じ人・同じ企業の passed は1行だけにする。
// 2回目以降を書くと、通過人数が応募人数を超えて通過率が 100% を超える。
// 記録済みなら INSERT を発行しない（sqlmock は予期しない INSERT で失敗する）。
func TestRecordAction_SkipsAlreadyRecordedPassed(t *testing.T) {
	s, mock := newDedupTestService(t)
	mock.ExpectQuery(consentQuery).WillReturnRows(sqlmock.NewRows([]string{"allow_collective_insight"}).AddRow(true))
	mock.ExpectQuery(hasActionQuery).WithArgs(anonymizeUserID(7), uint(10), "passed", 1).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	require.NoError(t, s.RecordAction(7, "sess-a", 10, "passed"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 閲覧(viewed)は繰り返し記録する（重複の確認をしない）。
func TestRecordAction_ViewedIsNotDeduplicated(t *testing.T) {
	s, mock := newDedupTestService(t)
	mock.ExpectQuery(consentQuery).WillReturnRows(sqlmock.NewRows([]string{"allow_collective_insight"}).AddRow(true))
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"})) // スコアのスナップショット取得
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `collective_insight_logs`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(t, s.RecordAction(7, "sess-a", 10, "viewed"))
	assert.NoError(t, mock.ExpectationsWereMet())
}
