package repositories_test

// UserWeightScoreRepository SetScore/AddScore のテスト（Issue #315）
// 実行: cd Backend && go test ./internal/repositories/... -run TestUserWeightScore -v

import (
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/internal/repositories"
)

func newScoreRepoTestDB(t *testing.T) (*repositories.UserWeightScoreRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock作成失敗: %v", err)
	}
	dialector := mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	})
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm open失敗: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return repositories.NewUserWeightScoreRepository(db), mock
}

// TestSetScore_CreatesNewRecord は SetScore が新規レコードを絶対値で作成することを検証する（#315修正）
func TestSetScore_CreatesNewRecord(t *testing.T) {
	repo, mock := newScoreRepoTestDB(t)

	mock.ExpectQuery("SELECT `organization_id` FROM `users` WHERE id = \\?").
		WithArgs(uint(1)).
		WillReturnRows(sqlmock.NewRows([]string{"organization_id"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `user_weight_scores`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := repo.SetScore(1, "session-1", "技術志向", 80)
	if err != nil {
		t.Errorf("SetScore returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations not met: %v", err)
	}
}

// TestAddScore_UpdatesExistingRecord は AddScore が既存レコードに差分加算することを検証する（#315修正）
func TestAddScore_UpdatesExistingRecord(t *testing.T) {
	repo, mock := newScoreRepoTestDB(t)

	mock.ExpectQuery("SELECT \\* FROM `user_weight_scores` WHERE").
		WithArgs(uint(1), "session-1", "技術志向", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "session_id", "weight_category", "score"}).
			AddRow(10, 1, "session-1", "技術志向", 70))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user_weight_scores` SET `score`=GREATEST\\(0, LEAST\\(100, score \\+ \\?\\)\\),`updated_at`=\\? WHERE `id` = \\?").
		WithArgs(5, sqlmock.AnyArg(), 10).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := repo.AddScore(1, "session-1", "技術志向", 5)
	if err != nil {
		t.Errorf("AddScore returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations not met: %v", err)
	}
}

// TestAddScore_RecordNotFound はレコードが存在しない場合にエラーを返すことを検証する
func TestAddScore_RecordNotFound(t *testing.T) {
	repo, mock := newScoreRepoTestDB(t)

	mock.ExpectQuery("SELECT \\* FROM `user_weight_scores` WHERE").
		WithArgs(uint(1), "session-1", "存在しないカテゴリ", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	err := repo.AddScore(1, "session-1", "存在しないカテゴリ", 5)
	if err == nil {
		t.Error("レコードが存在しない場合はエラーを返すべき")
	}
}

// TestSetScore_SemanticSeparation は SetScore と AddScore が意味論的に分離されていることを確認する（#315修正の設計検証）
// SetScore は CREATE、AddScore は UPDATE のみを実行する
func TestSetScore_SemanticSeparation(t *testing.T) {
	repo, mock := newScoreRepoTestDB(t)

	mock.ExpectQuery("SELECT `organization_id` FROM `users` WHERE id = \\?").
		WithArgs(uint(1)).
		WillReturnRows(sqlmock.NewRows([]string{"organization_id"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `user_weight_scores`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := repo.SetScore(1, "session-x", "安定志向", 50); err != nil {
		t.Errorf("SetScore returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("SetScore expectations: %v", err)
	}
}

// TestAddScore_ClampsToZeroOneHundred は加算結果が 0〜100 に丸められることを検証する
func TestAddScore_ClampsToZeroOneHundred(t *testing.T) {
	repo, mock := newScoreRepoTestDB(t)

	mock.ExpectQuery("SELECT \\* FROM `user_weight_scores` WHERE").
		WithArgs(uint(1), "session-1", "技術志向", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "session_id", "weight_category", "score"}).
			AddRow(10, 1, "session-1", "技術志向", 95))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user_weight_scores`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := repo.AddScore(1, "session-1", "技術志向", 20); err != nil {
		t.Errorf("AddScore returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations not met: %v", err)
	}
}

func TestRecalculationQueriesScopeCrossFeatureExclusionsToAppliedSession(t *testing.T) {
	repo, mock := newScoreRepoTestDB(t)

	mock.ExpectQuery(`(?s)SELECT COALESCE\(MAX\(uws\.id\).*uws\.session_id = \(.*prior\.updated_at <= er\.scored_at.*uws\.session_id = \(.*prior\.updated_at <= ir\.scores_applied_at.*WHERE uws\.calculation_version <> \?`).
		WithArgs("v2").
		WillReturnRows(sqlmock.NewRows([]string{"max_score_id", "eligible_sessions", "unsupported_sessions"}).
			AddRow(100, 2, 1))
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*prior\.updated_at <= er\.scored_at.*prior\.updated_at <= ir\.scores_applied_at`).
		WithArgs("v2").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
	mock.ExpectQuery(`(?s)SELECT MIN\(uws\.id\).*uws\.session_id = \(.*prior\.updated_at <= er\.scored_at.*uws\.session_id = \(.*prior\.updated_at <= ir\.scores_applied_at`).
		WithArgs("v2", uint(100), uint(0), 10).
		WillReturnRows(sqlmock.NewRows([]string{"score_id", "user_id", "session_id"}))

	if _, err := repo.RecalculationStats("v2"); err != nil {
		t.Fatalf("RecalculationStats returned error: %v", err)
	}
	if _, err := repo.FindNextRecalculationTargets("v2", 100, 0, 10); err != nil {
		t.Fatalf("FindNextRecalculationTargets returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// 対象を選んだ後で加点が反映されたセッションは、ロックしてから確かめ直して置換しない。
func TestReplaceChatSessionScores_RechecksEligibilityUnderLock(t *testing.T) {
	repo, mock := newScoreRepoTestDB(t)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `user_weight_scores` WHERE .* FOR UPDATE").
		WithArgs(uint(7), "session-1", "chat").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*touched\.updated_at >= ir\.scores_applied_at`).
		WithArgs(uint(7), "session-1", "v2").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectRollback()

	err := repo.ReplaceChatSessionScores(1, 7, "session-1", map[string]int{"技術志向": 80}, "v2")
	if !errors.Is(err, repositories.ErrRecalculationTargetChanged) {
		t.Fatalf("err = %v, want ErrRecalculationTargetChanged", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}
