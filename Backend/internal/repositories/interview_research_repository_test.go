package repositories_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"Backend/internal/repositories"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestInterviewResearchRepository_FindFreshは期限内だけ返す(t *testing.T) {
	db, mock := newTestDB(t)
	repo := repositories.NewInterviewResearchRepository(db)
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT .* FROM `interview_research_snapshots`").
		WithArgs("架空製作所", "エンジニア", now, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "company_key", "position", "questions_json", "fetched_at", "expires_at",
		}).AddRow(1, "架空製作所", "エンジニア", `["保存済み"]`, now.Add(-time.Hour), now.Add(time.Hour)))

	got, ok, err := repo.FindFresh(context.Background(), "架空製作所", "エンジニア", now)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []string{"保存済み"}, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 保存は INSERT ... ON DUPLICATE KEY UPDATE の1文で行う（SELECT してから INSERT しない）。
func TestInterviewResearchRepository_Saveは原子的なupsertで列幅に切り詰める(t *testing.T) {
	db, mock := newTestDB(t)
	repo := repositories.NewInterviewResearchRepository(db)
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	longKey := strings.Repeat("あ", 300)
	longPosition := strings.Repeat("い", 300)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `interview_research_snapshots`.*ON DUPLICATE KEY UPDATE `questions_json`=VALUES\\(`questions_json`\\),`fetched_at`=VALUES\\(`fetched_at`\\),`expires_at`=VALUES\\(`expires_at`\\),`updated_at`=VALUES\\(`updated_at`\\)").
		WithArgs(
			strings.Repeat("あ", 191), strings.Repeat("い", 191), `["質問"]`,
			now, now.Add(time.Hour), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(t, repo.Save(context.Background(), longKey, longPosition, []string{"質問"}, now, now.Add(time.Hour)))
	require.NoError(t, mock.ExpectationsWereMet())
}

// 検索側も保存と同じ幅に切り詰める。そうしないと長い企業名は保存済みでも見つからない。
func TestInterviewResearchRepository_FindFreshは保存と同じ幅で検索する(t *testing.T) {
	db, mock := newTestDB(t)
	repo := repositories.NewInterviewResearchRepository(db)
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT .* FROM `interview_research_snapshots`").
		WithArgs(strings.Repeat("あ", 191), "", now, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, ok, err := repo.FindFresh(context.Background(), strings.Repeat("あ", 300), "", now)
	require.NoError(t, err)
	require.False(t, ok)
	require.NoError(t, mock.ExpectationsWereMet())
}
