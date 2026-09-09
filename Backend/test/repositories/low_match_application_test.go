package repositories_test

// #1028 低マッチ応募の抽出条件を SQL レベルで固定する。
//
// 「低マッチのまま進行中の応募がある生徒」を教員へ出す機能なので、
// 条件を1つ落とすと出すべき生徒が消えるか、出すべきでない生徒が混ざる。
// どちらも画面上は自然に見えてしまい気づけない。

import (
	"regexp"
	"strings"
	"testing"

	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type capturedSQL struct{ got *[]string }

func (c capturedSQL) Match(_, actual string) error {
	*c.got = append(*c.got, strings.Join(strings.Fields(actual), " "))
	return nil
}

func newLowMatchRepo(t *testing.T) (*repositories.UserApplicationStatusRepository, *[]string) {
	t.Helper()
	got := &[]string{}
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(capturedSQL{got: got}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	mock.ExpectQuery("").WillReturnRows(sqlmock.NewRows([]string{"user_id", "company_name", "match_score", "status"}))
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return repositories.NewUserApplicationStatusRepository(db), got
}

func TestFindLowMatchApplicationsByUsers_SQL(t *testing.T) {
	repo, got := newLowMatchRepo(t)
	_, err := repo.FindLowMatchApplicationsByUsers([]uint{1, 2}, 40)
	require.NoError(t, err)
	require.Len(t, *got, 1)
	sql := (*got)[0]

	t.Run("境界値ちょうどを含めない", func(t *testing.T) {
		// 学生側の needsLowMatchConfirm は 40 未満で確認する。
		// ここを <= にすると「学生には確認が出ないのに教員一覧には出る」
		require.Regexp(t, regexp.MustCompile(`match_score\s*<\s*\?`), sql)
		require.NotRegexp(t, regexp.MustCompile(`match_score\s*<=\s*\?`), sql)
	})

	t.Run("終了済みの応募を除く", func(t *testing.T) {
		// 落選・辞退まで拾うと「今フォローすべき生徒」が埋もれる
		require.Contains(t, sql, "status NOT IN")
	})

	t.Run("マッチと企業を結合する", func(t *testing.T) {
		require.Contains(t, sql, "user_company_matches")
		require.Contains(t, sql, "companies")
	})

	t.Run("対象生徒で絞る", func(t *testing.T) {
		// 抜けると全生徒の応募を読み込む
		require.Contains(t, sql, "user_id IN")
	})
}

// 生徒が0人ならクエリを投げない。
func TestFindLowMatchApplicationsByUsers_EmptyInput(t *testing.T) {
	repo, got := newLowMatchRepo(t)
	res, err := repo.FindLowMatchApplicationsByUsers(nil, 40)
	require.NoError(t, err)
	require.Empty(t, res)
	require.Empty(t, *got, "生徒0人でクエリを投げている")
}
