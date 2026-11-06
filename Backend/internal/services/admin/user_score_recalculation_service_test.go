package admin

import (
	"context"
	"errors"
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
