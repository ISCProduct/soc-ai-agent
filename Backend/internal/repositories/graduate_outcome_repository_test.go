package repositories

import (
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// #2032: 学生向けの進路実績は学校で絞り、卒業生の氏名・学科・メモを読まない。
func TestGraduateOutcomeRepository_SQL(t *testing.T) {
	db, err := gorm.Open(
		mysql.New(mysql.Config{SkipInitializeWithVersion: true}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true},
	)
	if err != nil {
		t.Fatalf("DryRun DB: %v", err)
	}
	var sqls []string
	for _, kind := range []string{"query", "row"} {
		name := "capture_" + kind
		cb := func(tx *gorm.DB) { sqls = append(sqls, tx.Statement.SQL.String()) }
		if kind == "query" {
			err = db.Callback().Query().After("gorm:query").Register(name, cb)
		} else {
			err = db.Callback().Row().After("gorm:row").Register(name, cb)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	repo := NewGraduateOutcomeRepository(db)
	_, _ = repo.CountsByCompanyYear(5, GraduateOutcomeFilter{Year: 2025, Industry: "製造業"})
	_, _, _ = repo.FilterOptions(5)
	_, _, _ = repo.CompanyCounts(5, 1)
	if len(sqls) < 4 {
		t.Fatalf("SQL = %d 本: %v", len(sqls), sqls)
	}
	for _, s := range sqls {
		if !strings.Contains(s, "ge.school_id = ?") || !strings.Contains(s, "ge.deleted_at IS NULL") {
			t.Errorf("学校で絞っていない・削除済みを除いていない: %s", s)
		}
		for _, pii := range []string{"graduate_name", "department", "note"} {
			if strings.Contains(s, pii) {
				t.Errorf("卒業生の個人情報（%s）を読んでいる: %s", pii, s)
			}
		}
	}
}
