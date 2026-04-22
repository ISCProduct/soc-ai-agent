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
	// InterviewVideo は session_id IN ? では削除しない（#314修正済み）
	// user_id = ? による一括削除のみ（下記 expectDeleteByUserID にて検証）

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

// TestDeleteAccount_InterviewVideoDeletedByUserIDOnly は InterviewVideo が
// user_id = ? の1回のみ削除され、session_id IN ? による二重削除が発生しないことを検証する（#314修正の担保）
func TestDeleteAccount_InterviewVideoDeletedByUserIDOnly(t *testing.T) {
	db, mock := newAuthServiceTestDB(t)
	svc := &AuthService{db: db}

	// sqlmock は登録外のクエリが来るとエラーを返す（AnyOrder=false がデフォルト）
	// interview_videos に対して session_id IN ? が来れば unexpected query エラーになる

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE `users`.`id` = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs(uint(2), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).AddRow(2, "test@example.com"))

	// collectUserSessionIDs（全テーブル空返し）
	for _, tbl := range []string{"chat_messages", "user_weight_scores", "conversation_contexts",
		"ai_generated_questions", "user_analysis_progress", "user_embeddings",
		"user_company_matches", "variant_assignments", "resume_documents"} {
		mock.ExpectQuery("SELECT DISTINCT `session_id` FROM `" + tbl + "` WHERE user_id = \\?").
			WithArgs(uint(2)).
			WillReturnRows(sqlmock.NewRows([]string{"session_id"}))
	}

	// collectInterviewSessionIDs: interview_session あり
	mock.ExpectQuery("SELECT `id` FROM `interview_sessions` WHERE user_id = \\?").
		WithArgs(uint(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(20))

	// session_id IN ? で削除されるのは utterance と report のみ（video はない）
	mock.ExpectExec("DELETE FROM `realtime_usage_logs` WHERE interview_session_id IN \\(\\?\\)").
		WithArgs(20).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM `interview_utterances` WHERE session_id IN \\(\\?\\)").
		WithArgs(20).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM `interview_reports` WHERE session_id IN \\(\\?\\)").
		WithArgs(20).
		WillReturnResult(sqlmock.NewResult(0, 0))
	// ここに interview_videos の session_id IN ? は来ないはず

	// resume cascade（空）
	mock.ExpectQuery("SELECT `id` FROM `resume_documents` WHERE user_id = \\?").
		WithArgs(uint(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	// user_id = ? による一括削除（interview_videos は1回のみ）
	for _, tbl := range []string{"chat_messages", "ai_generated_questions", "conversation_contexts",
		"user_weight_scores", "user_analysis_progress", "user_embeddings",
		"variant_assignments", "user_company_matches", "user_application_statuses",
		"company_reviews", "resume_documents", "interview_sessions",
		"interview_videos", // ここで1回だけ削除される
		"realtime_usage_logs", "schedule_events", "skill_scores",
		"git_hub_repo_summaries", "git_hub_language_stats", "git_hub_repos", "git_hub_profiles"} {
		mock.ExpectExec("DELETE FROM `" + tbl + "` WHERE user_id = \\?").
			WithArgs(uint(2)).
			WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectExec("DELETE FROM `pending_registrations` WHERE email = \\?").
		WithArgs("test@example.com").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM `users` WHERE `users`.`id` = \\?").
		WithArgs(uint(2)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if err := svc.DeleteAccount(2); err != nil {
		t.Fatalf("DeleteAccount returned error: %v", err)
	}
	// 期待外クエリ（interview_videos WHERE session_id IN ?）が来ていないことを確認
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected SQL query detected（二重削除バグが再発した可能性）: %v", err)
	}
}
