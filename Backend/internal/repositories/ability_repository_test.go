package repositories

import (
	"strings"
	"testing"

	"Backend/internal/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	gomysql "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newAbilityRepo(t *testing.T) (*AbilityRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return NewAbilityRepository(db), mock
}

// 受検中の一意制約（uniq_ability_sessions_active）に負けたら、呼び出し側が読み直せる専用エラーにする。
func TestAbilityRepository_CreateSessionDuplicateActive(t *testing.T) {
	repo, mock := newAbilityRepo(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `ability_sessions`").
		WillReturnError(&gomysql.MySQLError{Number: 1062, Message: "Duplicate entry for key 'uniq_ability_sessions_active'"})
	mock.ExpectRollback()
	err := repo.CreateSession(&models.AbilitySession{PublicID: "p", UserID: 7, TemplateID: 1, TemplateVersion: 1, Status: models.AbilityStatusInProgress})
	require.ErrorIs(t, err, ErrAbilitySessionExists)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 自動保存の再送・選び直しは1行に畳む。INSERT だけだと UNIQUE 違反で2回目が落ちる。
func TestAbilityRepository_UpsertAnswerIsIdempotent(t *testing.T) {
	repo, mock := newAbilityRepo(t)
	for range 2 {
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO `ability_answers`.*ON DUPLICATE KEY UPDATE `choice`=VALUES\\(`choice`\\),`updated_at`=VALUES\\(`updated_at`\\)").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		require.NoError(t, repo.UpsertAnswer(3, 11, 2))
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

// 提出は「受検中のときだけ」通す条件付き UPDATE。同時に2回押されても片方だけが確定する。
func TestAbilityRepository_SubmitIsConditional(t *testing.T) {
	repo, mock := newAbilityRepo(t)
	sess := &models.AbilitySession{ID: 5, UserID: 7, Status: models.AbilityStatusInProgress}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `ability_sessions` SET .*WHERE id = \\? AND status = \\?").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	err := repo.Submit(sess, map[string]any{"verbal": map[string]int{"correct": 1, "total": 1}})
	require.ErrorIs(t, err, ErrAbilityAlreadySubmitted)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 結果は ability_results に入れる。user_weight_scores へは書かない
// （能力検査の正答率は10カテゴリへ変換できず、マッチングを歪める）。
func TestAbilityRepository_SubmitDoesNotTouchUserWeightScores(t *testing.T) {
	repo, mock := newAbilityRepo(t)
	sess := &models.AbilitySession{ID: 5, UserID: 7, Status: models.AbilityStatusInProgress}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `ability_sessions`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `ability_results`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// user_weight_scores への発行を期待に入れていないので、触れば ExpectationsWereMet が落ちる。
	require.NoError(t, repo.Submit(sess, map[string]any{"verbal": map[string]int{"correct": 1, "total": 1}}))
	require.NoError(t, mock.ExpectationsWereMet())
	require.Equal(t, models.AbilityStatusSubmitted, sess.Status)
}

// 他人の public_id は見つからない扱いにする（session_id だけで他人の受検を開けない）。
func TestAbilityRepository_FindSessionByPublicIDScopesToOwner(t *testing.T) {
	repo, mock := newAbilityRepo(t)
	mock.ExpectQuery("SELECT \\* FROM `ability_sessions` WHERE public_id = \\? AND user_id = \\?").
		WithArgs("p-1", uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "public_id", "user_id"}).AddRow(1, "p-1", 7))
	s, err := repo.FindSessionByPublicID(7, "p-1")
	require.NoError(t, err)
	require.Equal(t, uint(7), s.UserID)
	require.NoError(t, mock.ExpectationsWereMet())
}

// section 別の正答数は JSON として保存する。形が崩れたら読み出しで分かるようにする。
func TestAbilityRepository_FindResultRejectsBrokenJSON(t *testing.T) {
	repo, mock := newAbilityRepo(t)
	mock.ExpectQuery("SELECT \\* FROM `ability_results`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "session_id", "section_scores"}).AddRow(1, 5, "{"))
	_, err := repo.FindResult(5)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "unexpected end") || strings.Contains(err.Error(), "JSON"), "err=%v", err)
}
