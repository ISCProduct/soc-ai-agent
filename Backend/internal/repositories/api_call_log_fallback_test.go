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

// TestTotalFallbackCostSince_FiltersFallbackOnly はフォールバック専用予算の集計条件を固定する（#1293）。
//
// ここから via_fallback が落ちると、通常の OpenAI 利用（企業検索など）や
// 無料のローカル推論が同じ財布になり、ローカル障害時にフォールバックが
// 一度も発動しなくなる（実データで月 $57 に対し既定上限 $20）。
func TestTotalFallbackCostSince_FiltersFallbackOnly(t *testing.T) {
	var captured string
	matcher := sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		captured = actualSQL
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

	since := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(cost_usd\\), 0\\) FROM `api_call_logs`").
		WithArgs(true, since).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1.25))

	total, err := NewAPICallLogRepository(db).TotalFallbackCostSince(context.Background(), since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1.25 {
		t.Errorf("total = %v, want 1.25", total)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("期待したクエリ・束縛値と一致しない: %v", err)
	}
	if !strings.Contains(captured, "via_fallback = ?") {
		t.Errorf("via_fallback の絞り込みが無い: %s", captured)
	}
	if !strings.Contains(captured, "called_at >= ?") {
		t.Errorf("called_at の絞り込みが無い: %s", captured)
	}
}
