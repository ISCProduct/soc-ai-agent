package repositories

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// 担当校の一覧は、学校の学園と本人の学園が一致する行だけを返す（#1675）。
// school_id だけで絞ると、学校名を他学園の校名に書き替えた学生が混ざる。
// ListUsersPaged / ListStudentsPaged はどちらも担当校スコープから呼ばれるので両方を固定する。
func TestUserRepository_SchoolScopeRequiresSameOrganization(t *testing.T) {
	schoolID := uint(55)
	orgSubquery := "organization_id = \\(SELECT organization_id FROM schools WHERE id = \\?\\)"

	t.Run("ListUsersPaged", func(t *testing.T) {
		repo, mock := newLastActivityRepo(t)
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `users`.*school_id = \\?.*"+orgSubquery).
			WithArgs(schoolID, schoolID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery("SELECT \\* FROM `users`.*school_id = \\?.*"+orgSubquery).
			WithArgs(schoolID, schoolID, 10).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))

		_, _, err := repo.ListUsersPaged(10, 0, "", &schoolID)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ListStudentsPaged", func(t *testing.T) {
		repo, mock := newLastActivityRepo(t)
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `users`.*school_id = \\?.*"+orgSubquery).
			WithArgs("student", false, false, schoolID, schoolID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery("SELECT \\* FROM `users`.*school_id = \\?.*"+orgSubquery).
			WithArgs("student", false, false, schoolID, schoolID, 10).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))

		_, _, err := repo.ListStudentsPaged(10, 0, "", &schoolID)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
