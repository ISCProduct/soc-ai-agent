package repositories

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newSearchMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	return db, mock
}

// TestStudentSearchRepository_AlwaysAppliesVisibilityGuards は、
// どのフィルタ条件でも公開同意・未退会・学生ロールの絞り込みが必ず入ることを検証する（#1094 プライバシー要件）。
func TestStudentSearchRepository_AlwaysAppliesVisibilityGuards(t *testing.T) {
	tests := []struct {
		name    string
		filters StudentSearchFilters
	}{
		{name: "フィルタなし", filters: StudentSearchFilters{}},
		{name: "業界フィルタ", filters: StudentSearchFilters{IndustryID: 3}},
		{name: "勤務地フィルタ", filters: StudentSearchFilters{Location: "東京"}},
		{name: "スキルフィルタ", filters: StudentSearchFilters{Skill: "基本情報"}},
		{name: "タグフィルタ", filters: StudentSearchFilters{Tag: "即戦力"}},
		{name: "ID指定(セマンティック検索)", filters: StudentSearchFilters{UserIDs: []uint{1, 2}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newSearchMockDB(t)
			repo := NewStudentSearchRepository(db)

			// COUNT・SELECT の双方に公開同意ガードが乗ることを正規表現で強制する。
			guards := "allow_scout_visibility.*withdrawn_at IS NULL.*u.role.*is_guest"
			mock.ExpectQuery("SELECT count.*" + guards).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			mock.ExpectQuery(guards).
				WillReturnRows(sqlmock.NewRows([]string{"user_id", "name"}))

			_, _, err := repo.Search(7, tt.filters)
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestStudentSearchRepository_EmptyUserIDsReturnsNoRows は、
// セマンティック検索の候補が0件のとき全件返してしまわないことを検証する。
func TestStudentSearchRepository_EmptyUserIDsReturnsNoRows(t *testing.T) {
	db, mock := newSearchMockDB(t)
	repo := NewStudentSearchRepository(db)

	// 候補0件なら "1 = 0" が付き、必ず0件になる。
	mock.ExpectQuery("SELECT count.*1 = 0").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("1 = 0").WillReturnRows(sqlmock.NewRows([]string{"user_id"}))

	_, total, err := repo.Search(7, StudentSearchFilters{UserIDs: []uint{}})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEscapeLike(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "通常の文字列はそのまま", input: "東京", want: "東京"},
		{name: "パーセントをエスケープ", input: "100%", want: `100\%`},
		{name: "アンダースコアをエスケープ", input: "a_b", want: `a\_b`},
		{name: "バックスラッシュをエスケープ", input: `a\b`, want: `a\\b`},
		{name: "全件一致を狙う入力を無害化", input: "%", want: `\%`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, escapeLike(tt.input))
		})
	}
}

// TestStudentSearchRepository_VisibilitySQLContainsGuards はSQL文字列そのものを検査し、
// 将来リファクタでガード条件が落ちたら失敗させる。
func TestStudentSearchRepository_VisibilitySQLContainsGuards(t *testing.T) {
	db, _ := newSearchMockDB(t)
	repo := NewStudentSearchRepository(db)

	sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		r := &StudentSearchRepository{db: tx}
		return r.visibleStudents(7, StudentSearchFilters{}).Select("u.id").Find(&[]StudentSearchRow{})
	})
	_ = repo

	for _, guard := range []string{
		"allow_scout_visibility",
		"withdrawn_at IS NULL",
		"u.role",
		"is_guest",
	} {
		assert.True(t, regexp.MustCompile(regexp.QuoteMeta(guard)).MatchString(sql),
			"公開判定SQLに %q が含まれること: %s", guard, sql)
	}
}
