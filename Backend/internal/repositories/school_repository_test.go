package repositories

import (
	"testing"

	"Backend/internal/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newSchoolRepo(t *testing.T) (*SchoolRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return NewSchoolRepository(db), mock
}

// 学校名は本人の自己申告なので、学園で絞らないと他学園の school_id を自分に付けられる（#1675）。
// 完全一致・表記ゆれの両方の問い合わせに organization_id が入っていることを SQL で固定する。
func TestSchoolRepository_FindByNameIsScopedToOrganization(t *testing.T) {
	repo, mock := newSchoolRepo(t)

	mock.ExpectQuery("SELECT \\* FROM `schools` WHERE organization_id = \\? AND name = \\?").
		WithArgs(uint(7), "A校", 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery("SELECT \\* FROM `schools` WHERE organization_id = \\? AND status = \\?").
		WithArgs(uint(7), models.SchoolStatusActive).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	school, err := repo.FindByName("A校", 7)
	require.NoError(t, err)
	require.Nil(t, school, "別学園の校名は解決してはいけない")
	require.NoError(t, mock.ExpectationsWereMet())
}

// organization_id=0 は既定組織として扱う。users の保存側（CreateUser）と同じ正規化にしないと、
// 解決した学校と実際に保存される organization_id が食い違う。
func TestSchoolRepository_FindByNameTreatsZeroAsDefaultOrganization(t *testing.T) {
	repo, mock := newSchoolRepo(t)

	mock.ExpectQuery("SELECT \\* FROM `schools` WHERE organization_id = \\? AND name = \\?").
		WithArgs(models.DefaultOrganizationID, "既定校", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(101, "既定校"))

	school, err := repo.FindByName("既定校", 0)
	require.NoError(t, err)
	require.NotNil(t, school)
	require.Equal(t, uint(101), school.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}
