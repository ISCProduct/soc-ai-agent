package services

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newAuthServiceTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	dialector := mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	})
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	return db, mock
}

func expectSessionIDPluck(mock sqlmock.Sqlmock, table string) {
	mock.ExpectQuery("SELECT DISTINCT `session_id` FROM `" + table + "` WHERE user_id = \\?").
		WithArgs(uint(1)).
		WillReturnRows(sqlmock.NewRows([]string{"session_id"}))
}

func expectDeleteByUserID(mock sqlmock.Sqlmock, table string) {
	mock.ExpectExec("DELETE FROM `" + table + "` WHERE user_id = \\?").
		WithArgs(uint(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestDeleteAccount_DBNotConfigured(t *testing.T) {
	svc := &AuthService{}
	if err := svc.DeleteAccount(1); err == nil {
		t.Fatal("expected error when db is not configured")
	}
}

func TestDeleteAccount_CascadeDeleteQueries(t *testing.T) {
	db, mock := newAuthServiceTestDB(t)
	svc := &AuthService{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE `users`.`id` = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs(uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).AddRow(1, "user@example.com"))

	// collectUserSessionIDs
	mock.ExpectQuery("SELECT DISTINCT `session_id` FROM `chat_messages` WHERE user_id = \\?").
		WithArgs(uint(1)).
		WillReturnRows(sqlmock.NewRows([]string{"session_id"}).AddRow("session-1"))
	expectSessionIDPluck(mock, "user_weight_scores")
	expectSessionIDPluck(mock, "conversation_contexts")
	expectSessionIDPluck(mock, "ai_generated_questions")
	expectSessionIDPluck(mock, "user_analysis_progress")
	expectSessionIDPluck(mock, "user_embeddings")
	expectSessionIDPluck(mock, "user_company_matches")
	expectSessionIDPluck(mock, "variant_assignments")
	expectSessionIDPluck(mock, "resume_documents")

	mock.ExpectExec("DELETE FROM `session_validations` WHERE session_id IN \\(\\?\\)").
		WithArgs("session-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// interview cascade
	mock.ExpectQuery("SELECT `id` FROM `interview_sessions` WHERE user_id = \\?").
		WithArgs(uint(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))
	mock.ExpectExec("DELETE FROM `realtime_usage_logs` WHERE interview_session_id IN \\(\\?\\)").
		WithArgs(10).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `interview_utterances` WHERE session_id IN \\(\\?\\)").
		WithArgs(10).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `interview_reports` WHERE session_id IN \\(\\?\\)").
		WithArgs(10).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `interview_videos` WHERE session_id IN \\(\\?\\)").
		WithArgs(10).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// resume cascade
	mock.ExpectQuery("SELECT `id` FROM `resume_documents` WHERE user_id = \\?").
		WithArgs(uint(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(20))
	mock.ExpectQuery("SELECT `id` FROM `resume_reviews` WHERE document_id IN \\(\\?\\)").
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(30))
	mock.ExpectExec("DELETE FROM `resume_review_items` WHERE review_id IN \\(\\?\\)").
		WithArgs(30).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `resume_text_blocks` WHERE document_id IN \\(\\?\\)").
		WithArgs(20).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `resume_reviews` WHERE document_id IN \\(\\?\\)").
		WithArgs(20).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// user-linked tables
	expectDeleteByUserID(mock, "chat_messages")
	expectDeleteByUserID(mock, "ai_generated_questions")
	expectDeleteByUserID(mock, "conversation_contexts")
	expectDeleteByUserID(mock, "user_weight_scores")
	expectDeleteByUserID(mock, "user_analysis_progress")
	expectDeleteByUserID(mock, "user_embeddings")
	expectDeleteByUserID(mock, "variant_assignments")
	expectDeleteByUserID(mock, "user_company_matches")
	expectDeleteByUserID(mock, "user_application_statuses")
	expectDeleteByUserID(mock, "company_reviews")
	expectDeleteByUserID(mock, "resume_documents")
	expectDeleteByUserID(mock, "interview_sessions")
	expectDeleteByUserID(mock, "interview_videos")
	expectDeleteByUserID(mock, "realtime_usage_logs")
	expectDeleteByUserID(mock, "schedule_events")
	expectDeleteByUserID(mock, "skill_scores")
	expectDeleteByUserID(mock, "git_hub_repo_summaries")
	expectDeleteByUserID(mock, "git_hub_language_stats")
	expectDeleteByUserID(mock, "git_hub_repos")
	expectDeleteByUserID(mock, "git_hub_profiles")

	mock.ExpectExec("DELETE FROM `pending_registrations` WHERE email = \\?").
		WithArgs("user@example.com").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `users` WHERE `users`.`id` = \\?").
		WithArgs(uint(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := svc.DeleteAccount(1); err != nil {
		t.Fatalf("DeleteAccount returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}
