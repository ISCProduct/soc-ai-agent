package repositories

import (
	"context"
	"testing"
	"time"

	"Backend/internal/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newAIBudgetRepoTest(t *testing.T) (*AIBudgetRepository, sqlmock.Sqlmock) {
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
	return NewAIBudgetRepository(db), mock
}

func TestAIBudgetRepositoryUsageCosts(t *testing.T) {
	dayStart := time.Date(2026, 11, 5, 15, 0, 0, 0, time.UTC)
	monthStart := time.Date(2026, 10, 31, 15, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		scope  string
		column string
	}{
		{name: "利用者", scope: models.AIBudgetScopeUser, column: "user_id"},
		{name: "学校", scope: models.AIBudgetScopeSchool, column: "school_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newAIBudgetRepoTest(t)
			mock.ExpectQuery(`(?s)FROM api_call_logs\s+WHERE `+tt.column+` = \? AND called_at >= \?`).
				WithArgs(dayStart, uint(7), monthStart).
				WillReturnRows(sqlmock.NewRows([]string{"daily", "monthly"}).AddRow(1.5, 10.0))
			// 終了していないリアルタイム音声を見積もりに含めることを固定する。
			mock.ExpectQuery(`(?s)WHEN ended_at IS NULL THEN TIMESTAMPDIFF.*FROM realtime_usage_logs\s+WHERE `+tt.column+` = \? AND started_at >= \?`).
				WithArgs(dayStart, sqlmock.AnyArg(), 0.18, sqlmock.AnyArg(), 0.18, uint(7), monthStart).
				WillReturnRows(sqlmock.NewRows([]string{"daily", "monthly"}).AddRow(0.5, 2.0))

			daily, monthly, err := repo.UsageCosts(context.Background(), tt.scope, 7, dayStart, monthStart, 0.18)
			if err != nil {
				t.Fatalf("UsageCosts returned error: %v", err)
			}
			if daily != 2.0 || monthly != 12.0 {
				t.Fatalf("daily=%v monthly=%v", daily, monthly)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("expectations not met: %v", err)
			}
		})
	}
}

func TestAIBudgetRepositoryUsageCosts_不正なスコープは拒否する(t *testing.T) {
	repo, _ := newAIBudgetRepoTest(t)
	if _, _, err := repo.UsageCosts(context.Background(), "tenant", 7, time.Now(), time.Now(), 0.18); err == nil {
		t.Fatal("expected error")
	}
}
