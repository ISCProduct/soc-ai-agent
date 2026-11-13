package repositories

import (
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// #2030: ポートフォリオの材料は本人の分だけを引き、教員向けの詳細は読まない。
func TestPortfolioRepository_SQL(t *testing.T) {
	db, err := gorm.Open(
		mysql.New(mysql.Config{SkipInitializeWithVersion: true}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true},
	)
	if err != nil {
		t.Fatalf("DryRun DB: %v", err)
	}
	var sqls []string
	if err := db.Callback().Row().After("gorm:row").Register("capture_row", func(tx *gorm.DB) {
		sqls = append(sqls, tx.Statement.SQL.String())
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Query().After("gorm:query").Register("capture_query", func(tx *gorm.DB) {
		sqls = append(sqls, tx.Statement.SQL.String())
	}); err != nil {
		t.Fatal(err)
	}
	repo := NewPortfolioRepository(db)
	_, _ = repo.RecentInterviews(7, 3)
	_, _ = repo.RecentESReviews(7, 5)
	_, _ = repo.RecentAssessments(7, 3)
	if len(sqls) != 3 {
		t.Fatalf("SQL = %d 本: %v", len(sqls), sqls)
	}
	for _, s := range sqls {
		if !strings.Contains(s, "user_id = ?") {
			t.Errorf("本人で絞っていない: %s", s)
		}
		if strings.Contains(s, "teacher_report_json") {
			t.Errorf("教員向けの詳細を読んでいる: %s", s)
		}
	}
	if !strings.Contains(sqls[0], "deleted_at IS NULL") || !strings.Contains(sqls[0], "status = ?") {
		t.Errorf("削除済み・未終了の面接を除いていない: %s", sqls[0])
	}
}
