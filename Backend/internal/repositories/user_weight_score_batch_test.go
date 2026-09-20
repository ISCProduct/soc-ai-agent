package repositories

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newWeightScoreBatchRepo(t *testing.T) (*UserWeightScoreRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return NewUserWeightScoreRepository(db), mock
}

// TestFindLatestScoresByUsers_UsesLatestSession は「最新セッション」の定義を固定する（#1027）。
//
// チャット診断を面接スナップショットより優先し、同優先内では updated_at / id 降順。
func TestFindLatestScoresByUsers_UsesLatestSession(t *testing.T) {
	repo, mock := newWeightScoreBatchRepo(t)

	mock.ExpectQuery("ROW_NUMBER\\(\\) OVER \\(\\s*PARTITION BY user_id\\s*ORDER BY\\s*CASE WHEN session_id LIKE 'interview-%' THEN 1 ELSE 0 END ASC,\\s*updated_at DESC,\\s*id DESC\\s*\\)").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "weight_category", "score"}).
			AddRow(1, "技術志向", 90).
			AddRow(1, "成長志向", 70).
			AddRow(2, "安定志向", 60))

	got, err := repo.FindLatestScoresByUsers([]uint{1, 2})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet(), "最新セッションの選び方が変わっている")

	require.Len(t, got, 2)
	require.Equal(t, 90.0, got[1]["技術志向"])
	require.Equal(t, 70.0, got[1]["成長志向"])
	require.Equal(t, 60.0, got[2]["安定志向"])
}

// 空スライスではDBに触らない。IN () は構文エラーになるため。
func TestFindLatestScoresByUsers_EmptyInput(t *testing.T) {
	repo, mock := newWeightScoreBatchRepo(t)

	got, err := repo.FindLatestScoresByUsers(nil)
	require.NoError(t, err)
	require.Empty(t, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestListStudentsPaged_FiltersToStudents は、教員向け一覧が
// 管理者・ゲスト・退会者を含めないことを検証する（#1027）。
//
// 既存の ListUsersPaged は withdrawn_at しか見ておらず、
// そのまま使うと教員に管理者やゲストが「生徒」として並ぶ。
func TestListStudentsPaged_FiltersToStudents(t *testing.T) {
	repo, mock := newWeightScoreBatchRepo(t)
	userRepo := NewUserRepository(repo.db)

	// 生徒に限定する条件が Count と Find の両方に乗ること。
	guards := "withdrawn_at IS NULL.*role = .*is_guest = .*is_admin = "
	mock.ExpectQuery("SELECT count.*" + guards).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(guards).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	_, _, err := userRepo.ListStudentsPaged(25, 0, "", nil)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet(), "生徒への絞り込み条件が抜けている")
}

// 担当校の絞り込みが SQL に乗ること。抜けると他校の生徒が返る。
func TestListStudentsPaged_AppliesSchoolScope(t *testing.T) {
	repo, mock := newWeightScoreBatchRepo(t)
	userRepo := NewUserRepository(repo.db)
	schoolID := uint(7)

	mock.ExpectQuery("SELECT count.*school_id = ").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("school_id = ").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	_, _, err := userRepo.ListStudentsPaged(25, 0, "", &schoolID)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet(), "担当校の絞り込みが SQL に乗っていない")
}
