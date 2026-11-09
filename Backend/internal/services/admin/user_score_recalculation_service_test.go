package admin

import (
	"context"
	"errors"
	"flag"
	"os"
	"testing"

	"Backend/internal/models"
	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type spyRecalculationMatching struct {
	calls   int
	session string
	err     error
}

func (s *spyRecalculationMatching) CalculateMatching(_ context.Context, _ uint, sessionID string) error {
	s.calls++
	s.session = sessionID
	return s.err
}

func TestSaveRecalculatedScores(t *testing.T) {
	tests := []struct {
		name            string
		eligible        int
		matchingErr     error
		wantFailure     string
		wantMatchingRun int
	}{
		{name: "置換後に企業マッチングを作り直す", eligible: 1, wantMatchingRun: 1},
		{
			name: "マッチングの失敗は分類して残す", eligible: 1, matchingErr: errors.New("db down"),
			wantFailure: models.UserScoreRecalculationFailureMatching, wantMatchingRun: 1,
		},
		{
			name: "途中で加点されたら置換もマッチングもしない", eligible: 0,
			wantFailure: models.UserScoreRecalculationFailureTargetChanged,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock作成失敗: %v", err)
			}
			defer sqlDB.Close()
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
				&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatalf("gorm作成失敗: %v", err)
			}

			mock.ExpectBegin()
			mock.ExpectQuery("SELECT `id` FROM `user_weight_scores`").
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			mock.ExpectQuery(`SELECT COUNT\(\*\)`).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tt.eligible))
			if tt.eligible > 0 {
				mock.ExpectQuery("SELECT \\* FROM `user_weight_scores`").
					WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id"}).AddRow(1, 3))
				mock.ExpectExec("INSERT INTO `user_score_recalculation_snapshots`").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("DELETE FROM `user_weight_scores`").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("INSERT INTO `user_weight_scores`").WillReturnResult(sqlmock.NewResult(2, 1))
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}

			matching := &spyRecalculationMatching{err: tt.matchingErr}
			service := &UserScoreRecalculationService{
				scores:   repositories.NewUserWeightScoreRepository(db),
				matching: matching,
			}
			target := repositories.UserScoreRecalculationTarget{ScoreID: 1, UserID: 7, SessionID: "session-1"}

			got := service.saveRecalculatedScores(context.Background(), 1, target, map[string]int{"技術志向": 80}, "v2")
			if got != tt.wantFailure {
				t.Fatalf("failure = %q, want %q", got, tt.wantFailure)
			}
			if matching.calls != tt.wantMatchingRun {
				t.Fatalf("matching calls = %d, want %d", matching.calls, tt.wantMatchingRun)
			}
			if tt.wantMatchingRun > 0 && matching.session != "session-1" {
				t.Fatalf("matching session = %q", matching.session)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("expectations not met: %v", err)
			}
		})
	}
}

func newRecalculationServiceSQLMock(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock作成失敗: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm作成失敗: %v", err)
	}
	return db, mock
}

// 採点できる回答が無いセッションは、スコアを置き換えずに計算版だけを進め、次のジョブの
// 対象から外す。updated_at を動かすと最新セッションと加点の判定が変わるので、SET は
// calculation_version だけであることも確かめる。
func TestMarkUnscorableSession(t *testing.T) {
	tests := []struct {
		name        string
		eligible    int
		updateErr   error
		wantFailure string
	}{
		{name: "計算版だけを現行にして no_scores として数える", eligible: 1, wantFailure: models.UserScoreRecalculationFailureNoScores},
		{name: "途中で加点されたら計算版も変えない", eligible: 0, wantFailure: models.UserScoreRecalculationFailureTargetChanged},
		{name: "更新に失敗したら保存失敗として残す", eligible: 1, updateErr: errors.New("db down"), wantFailure: models.UserScoreRecalculationFailureSave},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newRecalculationServiceSQLMock(t)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT `id` FROM `user_weight_scores`").
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			mock.ExpectQuery(`SELECT COUNT\(\*\)`).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tt.eligible))
			if tt.eligible > 0 {
				update := mock.ExpectExec("^UPDATE `user_weight_scores` SET `calculation_version`=\\? WHERE user_id = \\? AND session_id = \\? AND source = \\?$").
					WithArgs("v2", uint(7), "session-1", models.ScoreSourceChat)
				if tt.updateErr != nil {
					update.WillReturnError(tt.updateErr)
					mock.ExpectRollback()
				} else {
					update.WillReturnResult(sqlmock.NewResult(0, 2))
					mock.ExpectCommit()
				}
			} else {
				mock.ExpectRollback()
			}

			service := &UserScoreRecalculationService{scores: repositories.NewUserWeightScoreRepository(db)}
			target := repositories.UserScoreRecalculationTarget{ScoreID: 1, UserID: 7, SessionID: "session-1"}

			if got := service.markUnscorableSession(1, target, "v2"); got != tt.wantFailure {
				t.Fatalf("failure = %q, want %q", got, tt.wantFailure)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("expectations not met: %v", err)
			}
		})
	}
}

// リースを延ばせなければ別のバッチが同じジョブを取得している。LLM を呼ぶ前に打ち切り、
// 自分のリースの解放だけを試みる。chat は nil なので、再計算に進むと panic する。
func TestRunBatchStopsBeforeLLMWhenLeaseIsLost(t *testing.T) {
	db, mock := newRecalculationServiceSQLMock(t)
	jobRows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "calculation_version", "status", "max_score_id", "cursor_score_id"}).
			AddRow(12, "v2", models.UserScoreRecalculationJobRunning, 100, 0)
	}
	mock.ExpectQuery("SELECT \\* FROM `user_score_recalculation_jobs`").WillReturnRows(jobRows())
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user_score_recalculation_jobs` SET `batch_lease_token`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(`SELECT MIN\(uws.id\) AS score_id`).
		WillReturnRows(sqlmock.NewRows([]string{"score_id", "user_id", "session_id"}).AddRow(5, 7, "session-1"))
	mock.ExpectQuery("SELECT \\* FROM `user_score_recalculation_jobs`").WillReturnRows(jobRows())
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user_score_recalculation_jobs` SET `batch_lease_until`=\\?,`updated_at`=\\? WHERE id = \\? AND batch_lease_token = \\?").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_score_recalculation_jobs`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user_score_recalculation_jobs` SET `batch_lease_token`=\\?,`batch_lease_until`=\\?,`updated_at`=\\? WHERE id = \\? AND batch_lease_token = \\?").
		WithArgs(nil, nil, sqlmock.AnyArg(), uint(12), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	service := &UserScoreRecalculationService{
		scores: repositories.NewUserWeightScoreRepository(db),
		jobs:   repositories.NewUserScoreRecalculationRepository(db),
	}
	if _, err := service.RunBatch(context.Background(), 12); !errors.Is(err, repositories.ErrUserScoreRecalculationBatchBusy) {
		t.Fatalf("err = %v, want %v", err, repositories.ErrUserScoreRecalculationBatchBusy)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

var updateDryRunGolden = flag.Bool("update", false, "ドライランのゴールデンファイルを更新する")

func intPtr(value int) *int { return &value }

// 表の形をゴールデンで固定する。更新: go test ./internal/services/admin/ -run DryRun -update
func TestFormatUserScoreRecalculationDryRun(t *testing.T) {
	out := FormatUserScoreRecalculationDryRun([]RecalculationDryRunSession{
		{ScoreID: 1, UserID: 7, Scores: []RecalculationDryRunScore{
			{Category: "チームワーク志向", Current: intPtr(50), Recalculated: intPtr(62)},
			{Category: "技術志向", Current: intPtr(80), Recalculated: intPtr(75)},
			{Category: "新しい軸", Recalculated: intPtr(40)},
			{Category: "消える軸", Current: intPtr(30)},
		}},
		{ScoreID: 2, UserID: 8, Failure: models.UserScoreRecalculationFailureLoad},
		{ScoreID: 3, UserID: 9},
	}) + FormatUserScoreRecalculationDryRun(nil)

	const golden = "testdata/user_score_recalculation_dryrun.golden"
	if *updateDryRunGolden {
		if err := os.WriteFile(golden, []byte(out), 0o644); err != nil {
			t.Fatalf("ゴールデンの更新に失敗: %v", err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("ゴールデンが読めない: %v", err)
	}
	if out != string(want) {
		t.Errorf("--- want ---\n%s\n--- got ---\n%s", want, out)
	}
}

func TestCompareRecalculatedScores(t *testing.T) {
	got := compareRecalculatedScores(map[string]int{"a": 1, "b": 2}, map[string]int{"b": 3, "c": 4})
	if len(got) != 3 || got[0].Category != "a" || got[0].Recalculated != nil ||
		*got[1].Current != 2 || *got[1].Recalculated != 3 || got[2].Current != nil {
		t.Fatalf("got %+v", got)
	}
}

func TestEstimateRecalculationLLMCalls(t *testing.T) {
	stats := repositories.UserScoreRecalculationStats{EligibleSessions: 3, EligibleAnswers: 40}
	tests := []struct {
		name         string
		reasonUsesAI bool
		topN         int
		want         uint
	}{
		{name: "AI理由なしは回答の評価だけ", topN: 20, want: 40},
		{name: "AI理由ありはセッションごとの上位N社を足す", reasonUsesAI: true, topN: 20, want: 100},
		{name: "上位0社なら足さない", reasonUsesAI: true, topN: 0, want: 40},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := estimateRecalculationLLMCalls(stats, tt.reasonUsesAI, tt.topN); got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}
