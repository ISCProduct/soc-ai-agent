package services

import (
	"context"
	"sync"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type mockObjectDeleter struct {
	mu   sync.Mutex
	keys []string
}

func (m *mockObjectDeleter) DeleteObject(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys = append(m.keys, key)
	return nil
}

func TestUserDeletionService_DeletesS3KeysAfterCommit(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}

	obj := &mockObjectDeleter{}
	svc := NewUserDeletionService(db, obj, nil)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE `users`.`id` = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs(uint(9), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).AddRow(9, "del@example.com"))
	mock.ExpectQuery("SELECT \\* FROM `interview_videos` WHERE user_id = \\?").
		WithArgs(uint(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "drive_file_id"}).
			AddRow(1, "interview-videos/1/a.webm"))
	mock.ExpectQuery("SELECT \\* FROM `resume_documents` WHERE user_id = \\?").
		WithArgs(uint(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "stored_path", "normalized_path", "annotated_path"}).
			AddRow(2, "s3://bucket/resumes/9/doc.pdf", "", ""))

	for _, tbl := range []string{"chat_messages", "user_weight_scores", "conversation_contexts",
		"ai_generated_questions", "user_analysis_progress", "user_embeddings",
		"user_company_matches", "variant_assignments", "resume_documents"} {
		mock.ExpectQuery("SELECT DISTINCT `session_id` FROM `" + tbl + "` WHERE user_id = \\?").
			WithArgs(uint(9)).
			WillReturnRows(sqlmock.NewRows([]string{"session_id"}))
	}
	mock.ExpectQuery("SELECT `id` FROM `interview_sessions` WHERE user_id = \\?").
		WithArgs(uint(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT `id` FROM `resume_documents` WHERE user_id = \\?").
		WithArgs(uint(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))
	mock.ExpectQuery("SELECT `id` FROM `resume_reviews` WHERE document_id IN \\(\\?\\)").
		WithArgs(2).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec("DELETE FROM `resume_text_blocks` WHERE document_id IN \\(\\?\\)").
		WithArgs(2).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM `resume_reviews` WHERE document_id IN \\(\\?\\)").
		WithArgs(2).
		WillReturnResult(sqlmock.NewResult(0, 0))

	for _, tbl := range []string{"chat_messages", "ai_generated_questions", "conversation_contexts",
		"user_weight_scores", "user_analysis_progress", "user_embeddings",
		"variant_assignments", "user_company_matches", "user_application_statuses",
		"company_reviews", "resume_documents", "interview_sessions",
		"interview_videos", "realtime_usage_logs", "schedule_events", "skill_scores",
		"git_hub_repo_summaries", "git_hub_language_stats", "git_hub_repos", "git_hub_profiles",
		"user_google_tokens"} {
		mock.ExpectExec("DELETE FROM `" + tbl + "` WHERE user_id = \\?").
			WithArgs(uint(9)).
			WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectExec("DELETE FROM `collective_insight_logs` WHERE anonymous_user_id = \\?").
		WithArgs(collectiveAnonymousHash(9)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM `pending_registrations` WHERE email = \\?").
		WithArgs("del@example.com").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM `users` WHERE `users`.`id` = \\?").
		WithArgs(uint(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := svc.DeleteUser(9, UserDeletionActor{Kind: "self"}); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}

	obj.mu.Lock()
	defer obj.mu.Unlock()
	if len(obj.keys) != 2 {
		t.Fatalf("expected 2 s3 deletes, got %v", obj.keys)
	}
	want := map[string]bool{"interview-videos/1/a.webm": true, "resumes/9/doc.pdf": true}
	for _, k := range obj.keys {
		if !want[k] {
			t.Fatalf("unexpected key %q in %v", k, obj.keys)
		}
	}
}
