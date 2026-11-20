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
// 控えはトランザクション内でロックして読み直し、退会と重なっても退会した学生のスコアを作り直さない。

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

func snapshotJSON(t *testing.T, rows []models.UserWeightScore) string {
	t.Helper()
	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var (
	scoreColumns    = []string{"id", "user_id", "session_id", "source", "weight_category", "score", "calculation_version", "created_at", "updated_at"}
	snapshotColumns = []string{"id", "job_id", "user_id", "session_id", "scores_json", "created_at"}
)

// userState は控えを読み直したあとのユーザー行の状態。
type userState int

const (
	userActive userState = iota
	userWithdrawn
	userGone
)

// expectLockedSnapshot は、控えをロックして読み直し、ユーザー行の状態を確かめる問い合わせ。
func expectLockedSnapshot(mock sqlmock.Sqlmock, scoresJSON string, state userState) {
	mock.ExpectQuery("SELECT \\* FROM `user_score_recalculation_snapshots` WHERE id = \\? .*FOR UPDATE").
		WithArgs(uint(9), 1).
		WillReturnRows(sqlmock.NewRows(snapshotColumns).AddRow(9, 1, 7, "s1", scoresJSON, time.Now()))
	users := sqlmock.NewRows([]string{"id", "withdrawn_at"})
	switch state {
	case userActive:
		users.AddRow(7, nil)
	case userWithdrawn:
		users.AddRow(7, time.Now())
	}
	mock.ExpectQuery("SELECT `id`,`withdrawn_at` FROM `users` WHERE id = \\?").
		WithArgs(uint(7), 1).
		WillReturnRows(users)
}

func TestRollbackSnapshot(t *testing.T) {
	t0 := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	t2 := t0.Add(2 * time.Hour)
	original := []models.UserWeightScore{
		{ID: 1, UserID: 7, SessionID: "s1", Source: models.ScoreSourceChat, WeightCategory: "技術志向", Score: 40, CalculationVersion: "v1", CreatedAt: t0, UpdatedAt: t1},
		{ID: 2, UserID: 7, SessionID: "s1", Source: models.ScoreSourceChat, WeightCategory: "成長志向", Score: 55, CalculationVersion: "v1", CreatedAt: t0, UpdatedAt: t2},
	}
	snapshot := models.UserScoreRecalculationSnapshot{ID: 9, JobID: 1, UserID: 7, SessionID: "s1"}

	t.Run("行がすでに無ければ、控えから元の時刻のまま書き戻して控えを消す", func(t *testing.T) {
		repo, mock := newRollbackMock(t)
		mock.ExpectBegin()
		expectLockedSnapshot(mock, snapshotJSON(t, original), userActive)
		mock.ExpectQuery("SELECT \\* FROM `user_weight_scores` WHERE .* FOR UPDATE").
			WithArgs(uint(7), "s1", models.ScoreSourceChat).
			WillReturnRows(sqlmock.NewRows(scoreColumns))
		// カテゴリごとに、点数・計算版・元の created_at / updated_at をそのまま書き戻す
		for _, row := range original {
			mock.ExpectExec("INSERT INTO `user_weight_scores`").
				// 列: organization_id, user_id, session_id, source, weight_category, score,
				// calculation_version, prompt_version, created_at, updated_at
				WithArgs(sqlmock.AnyArg(), row.UserID, row.SessionID, row.Source, row.WeightCategory, row.Score,
					row.CalculationVersion, sqlmock.AnyArg(), row.CreatedAt, row.UpdatedAt).
				WillReturnResult(sqlmock.NewResult(int64(row.ID+10), 1))
		}
		mock.ExpectExec("DELETE FROM `user_score_recalculation_snapshots`").WithArgs(uint(9)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		restored, err := repo.rollbackSnapshot(snapshot, "v2")
		if err != nil || !restored {
			t.Fatalf("restored = %v err = %v", restored, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("控えがもう無い（退会で消えた）なら何も書かない", func(t *testing.T) {
		repo, mock := newRollbackMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT \\* FROM `user_score_recalculation_snapshots` WHERE id = \\? .*FOR UPDATE").
			WillReturnRows(sqlmock.NewRows(snapshotColumns))
		mock.ExpectCommit()

		restored, err := repo.rollbackSnapshot(snapshot, "v2")
		if err != nil || restored {
			t.Fatalf("restored = %v err = %v", restored, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("スコアを書いていないこと: %v", err)
		}
	})

	t.Run("退会済み（保管期間中）なら点数を書き戻さず、控えも残す", func(t *testing.T) {
		repo, mock := newRollbackMock(t)
		mock.ExpectBegin()
		expectLockedSnapshot(mock, snapshotJSON(t, original), userWithdrawn)
		mock.ExpectCommit()

		restored, err := repo.rollbackSnapshot(snapshot, "v2")
		if err != nil || restored {
			t.Fatalf("restored = %v err = %v", restored, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("スコアも控えも触っていないこと: %v", err)
		}
	})

	t.Run("ユーザー行が無い（物理削除の後）なら点数を書き戻さず、取りこぼした控えを消す", func(t *testing.T) {
		repo, mock := newRollbackMock(t)
		mock.ExpectBegin()
		expectLockedSnapshot(mock, snapshotJSON(t, original), userGone)
		mock.ExpectExec("DELETE FROM `user_score_recalculation_snapshots`").WithArgs(uint(9)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		restored, err := repo.rollbackSnapshot(snapshot, "v2")
		if err != nil || restored {
			t.Fatalf("restored = %v err = %v", restored, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("控えだけを消し、スコアは書いていないこと: %v", err)
		}
	})

	t.Run("再計算の後に更新があれば戻さず、控えも消さない", func(t *testing.T) {
		repo, mock := newRollbackMock(t)
		mock.ExpectBegin()
		expectLockedSnapshot(mock, snapshotJSON(t, original), userActive)
		// 置換は元の updated_at（t1）を引き継ぐ。それより新しければ、その後にライブの採点があった
		mock.ExpectQuery("SELECT \\* FROM `user_weight_scores` WHERE .* FOR UPDATE").
			WillReturnRows(sqlmock.NewRows(scoreColumns).
				AddRow(5, 7, "s1", models.ScoreSourceChat, "技術志向", 70, "v2", t0, t1.Add(time.Hour)))
		mock.ExpectCommit()

		restored, err := repo.rollbackSnapshot(snapshot, "v2")
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
		expectLockedSnapshot(mock, snapshotJSON(t, original[:1]), userActive)
		mock.ExpectQuery("SELECT \\* FROM `user_weight_scores` WHERE .* FOR UPDATE").
			WillReturnRows(sqlmock.NewRows(scoreColumns).
				AddRow(5, 7, "s1", models.ScoreSourceChat, "技術志向", 70, "v2", t0, t1))
		mock.ExpectExec("DELETE FROM `user_weight_scores` WHERE id IN").WithArgs(uint(5)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("INSERT INTO `user_weight_scores`").WillReturnResult(sqlmock.NewResult(12, 1))
		mock.ExpectExec("DELETE FROM `user_score_recalculation_snapshots`").WithArgs(uint(9)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		restored, err := repo.rollbackSnapshot(snapshot, "v2")
		if err != nil || !restored {
			t.Fatalf("restored = %v err = %v", restored, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("控えも行も空なら控えだけ片付け、戻したことにしない（マッチングを作り直さない）", func(t *testing.T) {
		repo, mock := newRollbackMock(t)
		mock.ExpectBegin()
		expectLockedSnapshot(mock, "[]", userActive)
		mock.ExpectQuery("SELECT \\* FROM `user_weight_scores` WHERE .* FOR UPDATE").
			WillReturnRows(sqlmock.NewRows(scoreColumns))
		mock.ExpectExec("DELETE FROM `user_score_recalculation_snapshots`").WithArgs(uint(9)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		restored, err := repo.rollbackSnapshot(snapshot, "v2")
		if err != nil || restored {
			t.Fatalf("restored = %v err = %v", restored, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}
