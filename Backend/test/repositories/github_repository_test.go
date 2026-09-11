package repositories_test

import (
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newTestDB sqlmockを使ったGORMテスト用DBを返す
func newTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	dialector := mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	})
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	t.Cleanup(func() { sqlDB.Close() })
	return db, mock
}

// --- GetProfile ---

func TestGetProfile_NotFound(t *testing.T) {
	db, mock := newTestDB(t)
	repo := repositories.NewGitHubRepository(db)

	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	profile, err := repo.GetProfile(1)
	require.NoError(t, err)
	assert.Nil(t, profile, "should return nil when profile not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetProfile_Found(t *testing.T) {
	db, mock := newTestDB(t)
	repo := repositories.NewGitHubRepository(db)

	now := time.Now()
	cols := []string{"id", "user_id", "git_hub_login", "access_token", "total_contributions", "public_repos", "followers", "following", "synced_at", "created_at", "updated_at"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow(1, 1, "testuser", "token123", 100, 10, 5, 3, now, now, now))

	profile, err := repo.GetProfile(1)
	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, "testuser", profile.GitHubLogin)
	assert.Equal(t, "token123", profile.AccessToken)
	assert.Equal(t, 100, profile.TotalContributions)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- GetRepositories ---

func TestGetRepositories_Empty(t *testing.T) {
	db, mock := newTestDB(t)
	repo := repositories.NewGitHubRepository(db)

	cols := []string{"id", "user_id", "name", "full_name", "description", "language", "stars", "forks", "is_forked", "git_hub_updated_at", "created_at", "updated_at"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(cols))

	repos, err := repo.GetRepositories(1)
	require.NoError(t, err)
	assert.Empty(t, repos)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetRepositories_ReturnsSortedByStars(t *testing.T) {
	db, mock := newTestDB(t)
	repo := repositories.NewGitHubRepository(db)

	now := time.Now()
	cols := []string{"id", "user_id", "name", "full_name", "description", "language", "stars", "forks", "is_forked", "git_hub_updated_at", "created_at", "updated_at"}
	// DBからstar数降順で返ってくることを模擬（ORDER BY stars desc）
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow(1, 1, "top-repo", "user/top-repo", "", "Go", 100, 5, false, now, now, now).
			AddRow(2, 1, "low-repo", "user/low-repo", "", "Python", 3, 0, false, now, now, now))

	repos, err := repo.GetRepositories(1)
	require.NoError(t, err)
	require.Len(t, repos, 2)
	assert.Equal(t, "top-repo", repos[0].Name)
	assert.Equal(t, 100, repos[0].Stars)
	assert.Equal(t, "low-repo", repos[1].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- GetLanguageStats ---

func TestGetLanguageStats_Found(t *testing.T) {
	db, mock := newTestDB(t)
	repo := repositories.NewGitHubRepository(db)

	now := time.Now()
	cols := []string{"id", "user_id", "language", "bytes", "percentage", "created_at", "updated_at"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow(1, 1, "Go", 3, 60.0, now, now).
			AddRow(2, 1, "TypeScript", 2, 40.0, now, now))

	stats, err := repo.GetLanguageStats(1)
	require.NoError(t, err)
	require.Len(t, stats, 2)
	assert.Equal(t, "Go", stats[0].Language)
	assert.InDelta(t, 60.0, stats[0].Percentage, 0.01)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- ReplaceRepositories ---

func TestReplaceRepositories_Empty(t *testing.T) {
	db, mock := newTestDB(t)
	repo := repositories.NewGitHubRepository(db)

	// トランザクション: BEGIN → DELETE → COMMIT
	mock.ExpectBegin()
	mock.ExpectExec("DELETE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := repo.ReplaceRepositories(1, []models.GitHubRepo{})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReplaceRepositories_WithRepos(t *testing.T) {
	db, mock := newTestDB(t)
	repo := repositories.NewGitHubRepository(db)

	repos := []models.GitHubRepo{
		{UserID: 1, Name: "repo1", FullName: "user/repo1", Language: "Go"},
	}

	// トランザクション: BEGIN → DELETE → INSERT → COMMIT
	mock.ExpectBegin()
	mock.ExpectExec("DELETE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := repo.ReplaceRepositories(1, repos)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
