package repositories

import (
	"testing"
	"time"

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

// スコアの二重計上を防ぐ判定は、本人の中で本文のハッシュを引く（#1531）。
//
// 「反映済みか」で引くのが要点。保存済みかで引くと、診断セッションが無くて反映
// できなかった添削が「計上済み」と誤判定され、その本文が以後永久に乗らない。
// 自分自身に当たらないよう id < excludeID で絞る。
func TestESReviewRepository_HasScoredSameText(t *testing.T) {
	repo, mock := newESReviewRepo(t)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `es_reviews` WHERE user_id = \\? AND es_text_hash = \\? AND id < \\? AND scored_at IS NOT NULL").
		WithArgs(uint(9001), "abc123", uint(42), 1). // 末尾は Limit(1)
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	seen, err := repo.HasScoredSameText(9001, "abc123", 42)
	require.NoError(t, err)
	require.True(t, seen)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 直近の反映時刻も「反映済みの行」からだけ引く。
func TestESReviewRepository_LastScoredAt(t *testing.T) {
	repo, mock := newESReviewRepo(t)
	at := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT MAX\\(scored_at\\) AS scored_at FROM `es_reviews` WHERE user_id = \\? AND id < \\? AND scored_at IS NOT NULL").
		WithArgs(uint(9001), uint(42), 1).
		WillReturnRows(sqlmock.NewRows([]string{"scored_at"}).AddRow(at))

	got, err := repo.LastScoredAt(9001, 42)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 「新しい順」は作成時刻で決める。id だけで並べると、バックデートされた行の
// 表示順が仕様の文言とずれる。
func TestESReviewRepository_OrdersByCreatedAt(t *testing.T) {
	repo, mock := newESReviewRepo(t)

	mock.ExpectQuery("ORDER BY e.created_at DESC, e.id DESC").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err := repo.ListForSchoolStudent(9001, 101, 0)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
