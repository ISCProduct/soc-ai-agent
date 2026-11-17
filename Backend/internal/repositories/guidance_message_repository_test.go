package repositories

import (
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// #2028: 先生の受信一覧は担当校の学生に限り、既読は相手側の、表示した発言までに付ける。
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
	_ = db.Callback().Query().After("gorm:query").Register("capture_query", cb)
	_ = db.Callback().Update().After("gorm:update").Register("capture_update", cb)

	repo := NewGuidanceMessageRepository(db)
	school := uint(3)
	_, _ = repo.TeacherInbox(&school, 42, 30)
	_ = repo.MarkReadBy(1, "teacher", 9, time.Now())
	_, _ = repo.CountOpenQuestions(10)
	_, _ = repo.ListStudentThreads(10, &StudentThreadCursor{UpdatedAt: time.Now(), ID: 5}, 30)
	if len(sqls) != 4 {
		t.Fatalf("SQL = %v", sqls)
	}
	inbox := sqls[0]
	for _, want := range []string{
		"u.school_id = ?", "u.organization_id = (SELECT organization_id FROM schools WHERE id = ?)",
		// ResolveStudentSchool と同じ条件（開けない行を一覧に出さない）
		"u.role = ?", "u.is_admin = ?", "u.is_guest = ?", "u.withdrawn_at IS NULL",
		"HAVING MAX(m.id) < ?", "ORDER BY last_message_id DESC",
	} {
		if !strings.Contains(inbox, want) {
			t.Errorf("受信一覧に %s が無い: %s", want, inbox)
		}
	}
	for _, want := range []string{"author_role <> ?", "read_at IS NULL", "id <= ?"} {
		if !strings.Contains(sqls[1], want) {
			t.Errorf("既読に %s が無い: %s", want, sqls[1])
		}
	}
	if !strings.Contains(sqls[2], "g.kind = ?") || !strings.Contains(sqls[2], "NOT EXISTS") {
		t.Errorf("未回答の質問の数え方: %s", sqls[2])
	}
	if !strings.Contains(sqls[3], "g.student_user_id = ?") || !strings.Contains(sqls[3], "g.updated_at < ?") || strings.Contains(sqls[3], "dismissed_at") {
		t.Errorf("学生の相談一覧は本人の分を、閉じた案内も含めて返す: %s", sqls[3])
	}
}
