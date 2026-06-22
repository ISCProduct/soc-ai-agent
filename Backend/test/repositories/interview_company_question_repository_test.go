package repositories_test

import (
	"testing"

	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInterviewCompanyQuestionRepositoryFindByCompanyAndPosition(t *testing.T) {
	db, mock := newTestDB(t)
	repo := repositories.NewInterviewCompanyQuestionRepository(db)

	rows := sqlmock.NewRows([]string{
		"id", "company_id", "position", "category", "question_text", "priority", "is_required",
	}).
		AddRow(1, 42, "", "志望動機", "全職種向け質問", 1, true).
		AddRow(2, 42, "バックエンドエンジニア", "技術", "職種別質問", 2, false)

	mock.ExpectQuery("SELECT \\* FROM `interview_company_questions` WHERE company_id = \\? AND \\(position = '' OR position = \\?\\) ORDER BY priority ASC, id ASC").
		WithArgs(uint(42), "バックエンドエンジニア").
		WillReturnRows(rows)

	questions, err := repo.FindByCompanyAndPosition(42, "バックエンドエンジニア")

	require.NoError(t, err)
	require.Len(t, questions, 2)
	assert.Empty(t, questions[0].Position)
	assert.True(t, questions[0].IsRequired)
	assert.Equal(t, "バックエンドエンジニア", questions[1].Position)
	assert.False(t, questions[1].IsRequired)
	assert.NoError(t, mock.ExpectationsWereMet())
}
