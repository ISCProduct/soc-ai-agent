package repositories

import (
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// #2028: 先生の受信一覧は担当校の学生に限り、既読は相手側の発言だけに付ける。
func TestGuidanceMessageRepository_SQL(t *testing.T) {
	db, err := gorm.Open(
		mysql.New(mysql.Config{SkipInitializeWithVersion: true}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true},
	)
	if err != nil {
		t.Fatalf("DryRun DB: %v", err)
	}
	var sqls []string
	cb := func(tx *gorm.DB) { sqls = append(sqls, tx.Statement.SQL.String()) }
	_ = db.Callback().Row().After("gorm:row").Register("capture_row", cb)
	_ = db.Callback().Update().After("gorm:update").Register("capture_update", cb)

	repo := NewGuidanceMessageRepository(db)
	school := uint(3)
	_, _ = repo.TeacherInbox(&school, 100)
	_ = repo.MarkReadBy(1, "teacher", time.Now())
	if len(sqls) != 2 {
		t.Fatalf("SQL = %v", sqls)
	}
	if !strings.Contains(sqls[0], "u.school_id = ?") || !strings.Contains(sqls[0], "u.organization_id = (SELECT organization_id FROM schools WHERE id = ?)") {
		t.Errorf("受信一覧を担当校で絞っていない: %s", sqls[0])
	}
	if !strings.Contains(sqls[1], "author_role <> ?") || !strings.Contains(sqls[1], "read_at IS NULL") {
		t.Errorf("既読を相手側の未読の発言だけに付けていない: %s", sqls[1])
	}
}
