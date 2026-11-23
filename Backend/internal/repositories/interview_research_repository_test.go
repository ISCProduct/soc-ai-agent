package repositories_test

import (
	"context"
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
