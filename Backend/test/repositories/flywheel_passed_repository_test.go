package repositories_test

import (
	"database/sql/driver"
	"slices"
	"testing"

	"Backend/internal/models"
	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newFlywheelTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db, mock
}

func TestGetPassedApplicantScores_BindsCurrentAndLegacyStatuses(t *testing.T) {
	db, mock := newFlywheelTestDB(t)
	repo := repositories.NewProfileRecalculationRepository(db)

	args := []driver.Value{uint(7)}
	for _, s := range models.FlywheelPassedStatusFilter() {
		args = append(args, s)
	}

	cols := []string{
		"sample_count", "avg_technical", "avg_teamwork", "avg_leadership",
		"avg_creativity", "avg_stability", "avg_growth", "avg_work_life",
		"avg_challenge", "avg_detail", "avg_communication",
	}
	mock.ExpectQuery("(?i)FROM user_application_statuses").
		WithArgs(args...).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(3, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10))

	got, err := repo.GetPassedApplicantScores(7)
	require.NoError(t, err)
	assert.Equal(t, uint(7), got.CompanyID)
	assert.Equal(t, 3, got.SampleCount)
	assert.Contains(t, models.FlywheelPassedStatusFilter(), "interview_in_progress")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCategoryPassRateCorrelation_SQLIncludesInterviewInProgress(t *testing.T) {
	db, mock := newFlywheelTestDB(t)
	repo := repositories.NewScoreValidationRepository(db)

	cols := []string{"category", "score_band", "total_count", "pass_count", "avg_score"}
	mock.ExpectQuery("interview_in_progress").
		WillReturnRows(sqlmock.NewRows(cols).AddRow("技術志向", 4, 10, 3, 80))

	rows, err := repo.GetCategoryPassRateCorrelation()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 3, rows[0].PassCount)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFlywheelPassCount_InterviewInProgressIncrements_WithdrawnDoesNot(t *testing.T) {
	filter := models.FlywheelPassedStatusFilter()
	passCount := func(statuses []string) int {
		n := 0
		for _, s := range statuses {
			if slices.Contains(filter, s) {
				n++
			}
		}
		return n
	}

	base := []string{"applied", "rejected"}
	assert.Equal(t, 0, passCount(base))
	assert.Equal(t, 1, passCount(append(base, "interview_in_progress")))
	assert.Equal(t, 1, passCount(append(base, "document_passed")))
	assert.Equal(t, 1, passCount(append(base, "offered")))
	assert.Equal(t, 1, passCount(append(base, "accepted")))
	assert.Equal(t, 1, passCount(append(base, "interview")))
	assert.Equal(t, 0, passCount(append(base, "withdrawn")))
	assert.Equal(t, 0, passCount(append(base, "rejected")))
}
