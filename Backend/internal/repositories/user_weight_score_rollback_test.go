package repositories

import (
	"encoding/json"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/internal/models"
)

// SOCAIAGENT-630: ロールバックは、戻せたときだけ控えを消す。行がすでに無いセッションは
// 「再計算の後に更新された」のではないので、控えから書き戻す。

func newRollbackMock(t *testing.T) (*UserWeightScoreRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return NewUserWeightScoreRepository(db), mock
}

func rollbackSnapshotFixture(t *testing.T, rows []models.UserWeightScore) models.UserScoreRecalculationSnapshot {
	t.Helper()
	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	return models.UserScoreRecalculationSnapshot{ID: 9, JobID: 1, UserID: 7, SessionID: "s1", ScoresJSON: string(b)}
}

var scoreColumns = []string{"id", "user_id", "session_id", "source", "weight_category", "score", "calculation_version", "created_at", "updated_at"}

func TestRollbackSnapshot(t *testing.T) {
	t0 := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	original := []models.UserWeightScore{
		{ID: 1, UserID: 7, SessionID: "s1", Source: models.ScoreSourceChat, WeightCategory: "技術志向", Score: 40, CalculationVersion: "v1", CreatedAt: t0, UpdatedAt: t1},
	}

	t.Run("行がすでに無ければ、控えから書き戻して控えを消す", func(t *testing.T) {
		repo, mock := newRollbackMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT \\* FROM `user_weight_scores` WHERE .* FOR UPDATE").
			WithArgs(uint(7), "s1", models.ScoreSourceChat).
			WillReturnRows(sqlmock.NewRows(scoreColumns))
		mock.ExpectExec("INSERT INTO `user_weight_scores`").WillReturnResult(sqlmock.NewResult(11, 1))
		mock.ExpectExec("DELETE FROM `user_score_recalculation_snapshots`").WithArgs(uint(9)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		restored, err := repo.rollbackSnapshot(rollbackSnapshotFixture(t, original), "v2")
		if err != nil || !restored {
			t.Fatalf("restored = %v err = %v", restored, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("再計算の後に更新があれば戻さず、控えも消さない", func(t *testing.T) {
		repo, mock := newRollbackMock(t)
		mock.ExpectBegin()
		// 置換は元の updated_at（t1）を引き継ぐ。それより新しければ、その後にライブの採点があった
		mock.ExpectQuery("SELECT \\* FROM `user_weight_scores` WHERE .* FOR UPDATE").
			WillReturnRows(sqlmock.NewRows(scoreColumns).
				AddRow(5, 7, "s1", models.ScoreSourceChat, "技術志向", 70, "v2", t0, t1.Add(time.Hour)))
		mock.ExpectCommit()

		restored, err := repo.rollbackSnapshot(rollbackSnapshotFixture(t, original), "v2")
		if err != nil || restored {
			t.Fatalf("restored = %v err = %v", restored, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("控えを消していないこと: %v", err)
		}
	})

	t.Run("再計算のままなら元の行に戻し、控えを消す", func(t *testing.T) {
		repo, mock := newRollbackMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT \\* FROM `user_weight_scores` WHERE .* FOR UPDATE").
			WillReturnRows(sqlmock.NewRows(scoreColumns).
				AddRow(5, 7, "s1", models.ScoreSourceChat, "技術志向", 70, "v2", t0, t1))
		mock.ExpectExec("DELETE FROM `user_weight_scores` WHERE id IN").WithArgs(uint(5)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("INSERT INTO `user_weight_scores`").WillReturnResult(sqlmock.NewResult(12, 1))
		mock.ExpectExec("DELETE FROM `user_score_recalculation_snapshots`").WithArgs(uint(9)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		restored, err := repo.rollbackSnapshot(rollbackSnapshotFixture(t, original), "v2")
		if err != nil || !restored {
			t.Fatalf("restored = %v err = %v", restored, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}
