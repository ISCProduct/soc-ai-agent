package repositories

import (
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// #2039: ファネルの段階の SQL。削除済みの面接を数えず、面接は開始日時で期間を見る。
func TestConversionFunnel_InterviewExcludesDeletedAndUsesStartedAt(t *testing.T) {
	db, err := gorm.Open(
		mysql.New(mysql.Config{SkipInitializeWithVersion: true}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true},
	)
	if err != nil {
		t.Fatalf("DryRun DB: %v", err)
	}
	var sqls []string
	if err := db.Callback().Query().After("gorm:query").Register("capture_sql", func(tx *gorm.DB) {
		sqls = append(sqls, tx.Statement.SQL.String())
	}); err != nil {
		t.Fatalf("callback: %v", err)
	}
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	school := uint(3)
	if _, err := NewConversionFunnelRepository(db).StageUserIDs(&school, FunnelPeriod{From: &from}); err != nil {
		t.Fatalf("StageUserIDs: %v", err)
	}
	var interview string
	for _, s := range sqls {
		if strings.Contains(s, "interview_sessions") {
			interview = s
		}
	}
	if interview == "" {
		t.Fatalf("interview_sessions の SQL が無い: %v", sqls)
	}
	for _, want := range []string{"deleted_at IS NULL", "COALESCE(started_at, created_at) >=", "users.school_id"} {
		if !strings.Contains(interview, want) {
			t.Errorf("面接の SQL に %q が無い: %s", want, interview)
		}
	}
}
