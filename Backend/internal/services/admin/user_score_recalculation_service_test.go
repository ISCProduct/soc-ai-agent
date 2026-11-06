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
