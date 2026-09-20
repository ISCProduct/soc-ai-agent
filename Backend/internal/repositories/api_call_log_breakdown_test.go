package repositories

import (
	"context"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newMockRepo(t *testing.T) (*APICallLogRepository, sqlmock.Sqlmock, *string) {
	t.Helper()
	captured := new(string)
	matcher := sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		*captured = actualSQL
		return sqlmock.QueryMatcherRegexp.Match(expectedSQL, actualSQL)
	})
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewAPICallLogRepository(db), mock, captured
}

// TestUsageBreakdown_GroupsByDimension は集計軸ごとに正しい列で GROUP BY することを検証する（#1294）。
func TestUsageBreakdown_GroupsByDimension(t *testing.T) {
	tests := []struct {
		name       string
		dim        BreakdownDimension
		wantColumn string
	}{
		{"機能別", BreakdownByFeature, "feature"},
		{"プロバイダ別", BreakdownByProvider, "provider"},
		{"モデル別", BreakdownByModel, "model"},
		{"組織別", BreakdownByOrganization, "organization_id"},
	}

	since := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock, captured := newMockRepo(t)
			mock.ExpectQuery("FROM api_call_logs").
				WithArgs(since).
				WillReturnRows(sqlmock.NewRows([]string{"key", "total_cost_usd", "total_tokens", "call_count", "avg_latency_ms", "cache_hit_count"}).
					AddRow("es_review", 1.25, 1000, 3, 820.5, 1))

			rows, err := repo.UsageBreakdown(context.Background(), since, tt.dim)
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if len(rows) != 1 || rows[0].Key != "es_review" || rows[0].TotalCostUSD != 1.25 {
				t.Fatalf("集計結果のマッピングが不正: %+v", rows)
			}
			if rows[0].AvgLatencyMs != 820.5 || rows[0].CacheHitCount != 1 {
				t.Errorf("レイテンシ/キャッシュヒットが落ちている: %+v", rows[0])
			}
			if !strings.Contains(*captured, "GROUP BY "+tt.wantColumn) {
				t.Errorf("GROUP BY %s が無い: %s", tt.wantColumn, *captured)
			}
			if !strings.Contains(*captured, "called_at >= ?") {
				t.Errorf("期間の絞り込みが無い: %s", *captured)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("期待したクエリと一致しない: %v", err)
			}
		})
	}
}

// TestUsageBreakdown_RejectsUnknownDimension は許可リストに無い軸を拒否することを検証する（#1294）。
//
// 集計軸は SQL の列名として埋め込むため、クエリパラメータをそのまま通すと
// 任意の列を読み出せる。DBへ問い合わせる前にエラーにする。
func TestUsageBreakdown_RejectsUnknownDimension(t *testing.T) {
	tests := []struct {
		name string
		dim  BreakdownDimension
	}{
		{"未知の軸", "user_id"},
		{"空文字", ""},
		{"SQL片", "feature, (SELECT password FROM users LIMIT 1)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock, _ := newMockRepo(t)
			// クエリは1本も発行されないはず（ExpectQuery を登録しない）

			if _, err := repo.UsageBreakdown(context.Background(), time.Now(), tt.dim); err == nil {
				t.Fatal("未知の集計軸が通ってしまった")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("DBへ問い合わせてしまっている: %v", err)
			}
			if IsValidBreakdownDimension(tt.dim) {
				t.Errorf("IsValidBreakdownDimension(%q) が true", tt.dim)
			}
		})
	}
}
