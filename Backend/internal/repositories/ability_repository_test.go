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
		mock.ExpectExec("INSERT INTO `ability_answers`.*ON DUPLICATE KEY UPDATE").
			WillReturnResult(sqlmock.NewResult(1, 1))
		require.NoError(t, repo.UpsertAnswer(3, 11, 2))
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

// 保存は「受検中で、まだ締め切り前」を SQL の条件に入れた1文で行う。
//
// サービス側で確認してから書く形だと、確認と書き込みの間に提出が確定したり
// 締め切りを過ぎたりしたとき、採点に入らない回答が保存される。
// 条件を SELECT 側に持たせ、合わなければ 0 行で返る形になっていることを見る。
func TestAbilityRepository_UpsertAnswerIsConditional(t *testing.T) {
	repo, mock := newAbilityRepo(t)
	mock.ExpectExec("INSERT INTO `ability_answers`.*FROM `ability_sessions`.*WHERE s.id = \\? AND s.status = \\? AND \\(s.deadline_at IS NULL OR s.deadline_at >= \\?\\)").
		WillReturnResult(sqlmock.NewResult(0, 0))
	err := repo.UpsertAnswer(3, 11, 2)
	require.ErrorIs(t, err, ErrAbilityAnswerRejected)
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
	err := repo.Submit(sess, func(map[uint]int) (map[string]any, error) {
		t.Error("状態を切り替えられなかったのに採点した")
		return nil, nil
	})
	require.ErrorIs(t, err, ErrAbilityAlreadySubmitted)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 採点は「提出済みへ切り替えたあとに読んだ回答」で行う。
//
// 先に採点する形だと、切り替え前に確定した回答を取りこぼしたり、
// 切り替え後に保存された回答が採点に入らないまま残ったりする。
// UPDATE → SELECT(answers) → INSERT(results) の順になっていることを見る。
func TestAbilityRepository_SubmitScoresInsideTransaction(t *testing.T) {
	repo, mock := newAbilityRepo(t)
	sess := &models.AbilitySession{ID: 5, UserID: 7, Status: models.AbilityStatusInProgress}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `ability_sessions`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT \\* FROM `ability_answers` WHERE session_id = \\?").
		WithArgs(uint(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "session_id", "question_id", "choice"}).
			AddRow(1, 5, 11, 2).AddRow(2, 5, 12, 0))
	mock.ExpectExec("INSERT INTO `ability_results`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	var got map[uint]int
	err := repo.Submit(sess, func(answers map[uint]int) (map[string]any, error) {
		got = answers
		return map[string]any{"verbal": map[string]int{"correct": 1, "total": 2}}, nil
	})
	require.NoError(t, err)
	require.Equal(t, map[uint]int{11: 2, 12: 0}, got, "トランザクション内で読んだ回答が採点へ渡る")
	require.NoError(t, mock.ExpectationsWereMet())
}

// 結果は ability_results に入れる。user_weight_scores へは書かない
// （能力検査の正答率は10カテゴリへ変換できず、マッチングを歪める）。
func TestAbilityRepository_SubmitDoesNotTouchUserWeightScores(t *testing.T) {
	repo, mock := newAbilityRepo(t)
	sess := &models.AbilitySession{ID: 5, UserID: 7, Status: models.AbilityStatusInProgress}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `ability_sessions`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT \\* FROM `ability_answers`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "session_id", "question_id", "choice"}))
	mock.ExpectExec("INSERT INTO `ability_results`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// user_weight_scores への発行を期待に入れていないので、触れば ExpectationsWereMet が落ちる。
	require.NoError(t, repo.Submit(sess, func(map[uint]int) (map[string]any, error) {
		return map[string]any{"verbal": map[string]int{"correct": 1, "total": 1}}, nil
	}))
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
