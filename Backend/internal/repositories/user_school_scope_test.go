package repositories

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return db, mock
}

// 担当校で絞るクエリは、学校の学園と本人の学園の一致も要求する（#1675）。
// 素の school_id だけで絞ると、プロフィールの学校名（自己申告）を他学園の校名に
// 書き替えた学生が、その学校の職員の画面に現れる。
// 担当校スコープ（EchoAdminSchoolScope）を受ける経路すべてを固定する。
// 1つでも漏らすと、同じ穴がその経路に残る。
func TestSchoolScopedQueries_RequireSameOrganization(t *testing.T) {
	schoolID := uint(55)
	orgSub := `users\.organization_id = \(SELECT organization_id FROM schools WHERE id = \?\)`

	t.Run("ユーザー一覧", func(t *testing.T) {
		db, mock := newMockDB(t)
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `users`.*users\\.school_id = \\?.*"+orgSub).
			WithArgs(schoolID, schoolID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery("SELECT \\* FROM `users`.*users\\.school_id = \\?.*"+orgSub).
			WithArgs(schoolID, schoolID, 10).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))

		_, _, err := NewUserRepository(db).ListUsersPaged(10, 0, "", &schoolID)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("生徒一覧", func(t *testing.T) {
		db, mock := newMockDB(t)
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `users`.*users\\.school_id = \\?.*"+orgSub).
			WithArgs("student", false, false, schoolID, schoolID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery("SELECT \\* FROM `users`.*users\\.school_id = \\?.*"+orgSub).
			WithArgs("student", false, false, schoolID, schoolID, 10).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))

		_, _, err := NewUserRepository(db).ListStudentsPaged(10, 0, "", &schoolID)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	// 応募一覧（FindAll）と1件ごとの認可判定（FindOwnerSchoolID）は
	// application_status_scope_test.go が同じ条件を固定しているので、ここでは重ねない。
	t.Run("面接集計", func(t *testing.T) {
		db, mock := newMockDB(t)
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `interview_sessions`.*JOIN users.*users\\.school_id = \\?.*"+orgSub).
			WithArgs(schoolID, schoolID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		_, err := NewInterviewSessionRepository(db).CountAll(&schoolID, nil)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
