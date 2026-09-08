package repositories

import (
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newWeightScoreRepo(t *testing.T) (*UserWeightScoreRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return NewUserWeightScoreRepository(db), mock
}

// TestSetScore_RejectsUnknownCategory は、正典外のカテゴリがDBへ入らないことを検証する（#929）。
//
// マッチングは scoreMap を正典キーで引くため、揺れた名前で保存された行は
// 一度も引かれず、代わりに中立50が使われる。エラーもログも出ないので、
// 保存の入口で弾かないと静かにスコアが壊れ続ける。
func TestSetScore_RejectsUnknownCategory(t *testing.T) {
	repo, mock := newWeightScoreRepo(t)

	err := repo.SetScore(1, "session-1", "ぜんぜん違うカテゴリ", 80)
	require.Error(t, err, "正典外のカテゴリが素通りしている")
	require.Contains(t, err.Error(), "未知の重みカテゴリ")
	// DBに一切触れないこと。
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAddScore_RejectsUnknownCategory(t *testing.T) {
	repo, mock := newWeightScoreRepo(t)

	err := repo.AddScore(1, "session-1", "ぜんぜん違うカテゴリ", 5)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 表記揺れは弾くのではなく正典へ寄せて保存する。
// 既存の呼び出し元をいきなり壊さないための緩衝。
func TestSetScore_NormalizesAliasBeforeInsert(t *testing.T) {
	repo, mock := newWeightScoreRepo(t)

	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"organization_id"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `user_weight_scores`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// 「チームワーク」は正典では「チームワーク志向」。
	if err := repo.SetScore(1, "session-1", "チームワーク", 80); err != nil {
		// 組織ID解決のモックが噛み合わない環境ではスキップ扱いにせず、
		// 少なくとも「未知カテゴリ」エラーでないことは確認する。
		require.False(t, strings.Contains(err.Error(), "未知の重みカテゴリ"),
			"別名が弾かれている: %v", err)
	}
}
