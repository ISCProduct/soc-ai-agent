package repositories

import (
	"context"
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
	ctx := context.Background()
	school := uint(3)
	for _, tt := range []struct {
		name    string
		op      func()
		want    []string
		notWant []string
	}{
		{
			name: "受信一覧は担当校の学生に限る",
			op:   func() { _, _ = repo.TeacherInbox(ctx, &school, 42, 30) },
			want: []string{
				"u.school_id = ?", "u.organization_id = (SELECT organization_id FROM schools WHERE id = ?)",
				// ResolveStudentSchool と同じ条件（開けない行を一覧に出さない）
				"u.role = ?", "u.is_admin = ?", "u.is_guest = ?", "u.withdrawn_at IS NULL",
				"HAVING MAX(m.id) < ?", "ORDER BY last_message_id DESC",
			},
		},
		{
			name: "既読は相手側の、返した発言の ID だけ",
			op:   func() { _ = repo.MarkReadBy(ctx, 1, "teacher", []uint{7, 9}, time.Now()) },
			want: []string{"author_role <> ?", "read_at IS NULL", "id IN (?,?)"},
		},
		{
			name: "未回答の質問を数える",
			op:   func() { _, _ = repo.CountOpenQuestions(ctx, 10) },
			want: []string{"g.kind = ?", "NOT EXISTS"},
		},
		{
			name: "学生の相談一覧は本人の分を、閉じた案内も含めて返す",
			op: func() {
				_, _ = repo.ListStudentThreads(ctx, 10, &StudentThreadCursor{UpdatedAt: time.Now(), ID: 5}, 30)
			},
			want:    []string{"g.student_user_id = ?", "g.updated_at < ?"},
			notWant: []string{"dismissed_at"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sqls = nil
			tt.op()
			if len(sqls) != 1 {
				t.Fatalf("SQL = %v", sqls)
			}
			for _, w := range tt.want {
				if !strings.Contains(sqls[0], w) {
					t.Errorf("%s が無い: %s", w, sqls[0])
				}
			}
			for _, w := range tt.notWant {
				if strings.Contains(sqls[0], w) {
					t.Errorf("%s がある: %s", w, sqls[0])
				}
			}
		})
	}
}
