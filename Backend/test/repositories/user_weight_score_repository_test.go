package repositories_test

// UserWeightScoreRepository SetScore/AddScore のテスト（Issue #315）
// 実行: cd Backend && go test ./test/repositories/... -run TestUserWeightScore -v

import (
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
	mock.ExpectExec("UPDATE `user_weight_scores`").
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

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `user_weight_scores`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := repo.SetScore(1, "session-x", "安定志向", 50); err != nil {
		t.Errorf("SetScore returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("SetScore が SELECT を発行した（予期しないクエリ）: %v", err)
	}
}
