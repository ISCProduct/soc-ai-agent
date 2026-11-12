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

// aiBudgetUsageQuery は利用額の集計 SQL の要所を固定する。
//   - 終了していない音声の日次は当日0時（GREATEST(started_at, 当日0時)）から数える
//   - 経過秒は 0 未満にしない
//   - 対象は IN でまとめて読む
func aiBudgetUsageQuery(column string) string {
	return `(?s)FROM api_call_logs\s+WHERE ` + column + ` IN \(\?.*\) AND called_at >= \?` +
		`.*GREATEST\(TIMESTAMPDIFF\(SECOND, GREATEST\(started_at, CAST\(\? AS DATETIME\(3\)\)\), \?\), 0\)` +
		`.*WHEN started_at >= \? THEN cost_usd` +
		`.*GREATEST\(TIMESTAMPDIFF\(SECOND, started_at, \?\), 0\)` +
		`.*FROM realtime_usage_logs\s+WHERE ` + column + ` IN \(\?.*\) AND started_at >= \?` +
		`.*GROUP BY scope_id`
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
			mock.ExpectQuery(aiBudgetUsageQuery(tt.column)).
				WithArgs(
					dayStart, uint(7), monthStart,
					dayStart, sqlmock.AnyArg(), 0.18, dayStart,
					sqlmock.AnyArg(), 0.18,
					uint(7), monthStart,
				).
				WillReturnRows(sqlmock.NewRows([]string{"scope_id", "daily", "monthly"}).AddRow(7, 2.0, 12.0))

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

// 管理画面の一覧は、ページの対象を1本の集計で読む。利用のない対象はゼロ値になる。
func TestAIBudgetRepositoryUsageCostsByScope(t *testing.T) {
	dayStart := time.Date(2026, 11, 5, 15, 0, 0, 0, time.UTC)
	monthStart := time.Date(2026, 10, 31, 15, 0, 0, 0, time.UTC)
	repo, mock := newAIBudgetRepoTest(t)
	mock.ExpectQuery(aiBudgetUsageQuery("school_id")).
		WithArgs(
			dayStart, uint(3), uint(4), uint(5), monthStart,
			dayStart, sqlmock.AnyArg(), 0.18, dayStart,
			sqlmock.AnyArg(), 0.18,
			uint(3), uint(4), uint(5), monthStart,
		).
		WillReturnRows(sqlmock.NewRows([]string{"scope_id", "daily", "monthly"}).
			AddRow(3, 1.5, 10.0).
			AddRow(5, 0.0, 4.0))

	usage, err := repo.UsageCostsByScope(context.Background(), models.AIBudgetScopeSchool, []uint{3, 4, 5}, dayStart, monthStart, 0.18)
	if err != nil {
		t.Fatalf("UsageCostsByScope returned error: %v", err)
	}
	if usage[3] != (AIBudgetUsage{Daily: 1.5, Monthly: 10}) || usage[4] != (AIBudgetUsage{}) || usage[5] != (AIBudgetUsage{Monthly: 4}) {
		t.Fatalf("usage=%+v", usage)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

func TestAIBudgetRepositoryUsageCostsByScope_対象が無ければ読まない(t *testing.T) {
	repo, mock := newAIBudgetRepoTest(t)
	usage, err := repo.UsageCostsByScope(context.Background(), models.AIBudgetScopeUser, nil, time.Now(), time.Now(), 0.18)
	if err != nil || len(usage) != 0 {
		t.Fatalf("usage=%v err=%v", usage, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

func TestAIBudgetRepositoryUsageCosts_不正なスコープは拒否する(t *testing.T) {
	repo, _ := newAIBudgetRepoTest(t)
	if _, _, err := repo.UsageCosts(context.Background(), "tenant", 7, time.Now(), time.Now(), 0.18); err == nil {
		t.Fatal("expected error")
	}
}
