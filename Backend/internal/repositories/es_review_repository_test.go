package repositories

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newESReviewRepo(t *testing.T) (*ESReviewRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return NewESReviewRepository(db), mock
}

// 教員向けの履歴は、学校だけで絞ってはいけない（#1531 / #1675）。
// users.school_id はプロフィールの学校名（自己申告）から解決されるため、
// 学校名を他学園の校名に書き替えた学生の履歴がその学校のものとして見えてしまう。
// 学校の学園・本人の学園・保存時点の学園の3つが揃うことを SQL で固定する。
func TestESReviewRepository_ListForSchoolStudentRequiresSameOrganization(t *testing.T) {
	repo, mock := newESReviewRepo(t)

	mock.ExpectQuery("SELECT e\\.\\* FROM es_reviews AS e.*"+
		"JOIN users u ON u\\.id = e\\.user_id.*"+
		"JOIN schools s ON s\\.id = u\\.school_id AND s\\.organization_id = u\\.organization_id.*"+
		"e\\.organization_id = u\\.organization_id").
		WithArgs(uint(9001), uint(101), "student", false, false, 50).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err := repo.ListForSchoolStudent(9001, 101, 0)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 前回の引き当ては、保存した自分自身を除く。
// 除かないと、同一設問を保存した直後に自分が「前回」になり差分が常にゼロになる。
func TestESReviewRepository_FindPreviousExcludesSelf(t *testing.T) {
	repo, mock := newESReviewRepo(t)

	mock.ExpectQuery("SELECT \\* FROM `es_reviews` WHERE user_id = \\? AND question_key = \\? AND id < \\?").
		WithArgs(uint(9001), "ガクチカの設問", uint(42), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))

	got, err := repo.FindPreviousByQuestion(9001, "ガクチカの設問", 42)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, uint(7), got.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 前回が無いのはエラーではない（初回の添削）。
func TestESReviewRepository_FindPreviousReturnsNilWhenAbsent(t *testing.T) {
	repo, mock := newESReviewRepo(t)
	mock.ExpectQuery("SELECT \\* FROM `es_reviews`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	got, err := repo.FindPreviousByQuestion(9001, "初回", 1)
	require.NoError(t, err)
	require.Nil(t, got)
}
