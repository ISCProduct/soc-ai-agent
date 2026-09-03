package auth

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type scoutSyncerRecorder struct {
	userIDs []uint
}

func (s *scoutSyncerRecorder) Sync(_ context.Context, userID uint) {
	s.userIDs = append(s.userIDs, userID)
}

// TestDeleteUser_SyncsScoutIndex は、退会時にスカウト検索インデックスの
// 同期（= 公開対象外になるためベクトル削除）が呼ばれることを検証する (#1094)。
// これが無いと、退会後も学生プロフィールのベクトルが RAG 側に残り続ける。
func TestDeleteUser_SyncsScoutIndex(t *testing.T) {
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

	spy := &scoutSyncerRecorder{}
	svc := NewUserDeletionService(db, &mockObjectDeleter{}, nil)
	svc.SetScoutIndexSyncer(spy)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE `users`.`id` = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs(uint(9), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "withdrawn_at"}).AddRow(9, "del@example.com", nil))
	mock.ExpectQuery("SELECT \\* FROM `interview_videos` WHERE user_id = \\?").
		WithArgs(uint(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "drive_file_id"}))
	mock.ExpectQuery("SELECT \\* FROM `resume_documents` WHERE user_id = \\?").
		WithArgs(uint(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "stored_path", "normalized_path", "annotated_path"}))
	mock.ExpectExec("INSERT INTO `withdrawn_users`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE `users` SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `user_refresh_tokens` SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := svc.DeleteUser(9, UserDeletionActor{Kind: "self"}); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}

	if len(spy.userIDs) != 1 || spy.userIDs[0] != 9 {
		t.Fatalf("退会時にスカウトインデックスが同期されていない: %v", spy.userIDs)
	}
}

// TestDeleteUser_WithoutScoutSyncer は、同期処理が未設定（RAG未設定環境）でも
// 退会が成功することを検証する。
func TestDeleteUser_WithoutScoutSyncer(t *testing.T) {
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

	svc := NewUserDeletionService(db, &mockObjectDeleter{}, nil)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE `users`.`id` = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs(uint(9), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "withdrawn_at"}).AddRow(9, "del@example.com", nil))
	mock.ExpectQuery("SELECT \\* FROM `interview_videos` WHERE user_id = \\?").
		WithArgs(uint(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "drive_file_id"}))
	mock.ExpectQuery("SELECT \\* FROM `resume_documents` WHERE user_id = \\?").
		WithArgs(uint(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "stored_path", "normalized_path", "annotated_path"}))
	mock.ExpectExec("INSERT INTO `withdrawn_users`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE `users` SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `user_refresh_tokens` SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := svc.DeleteUser(9, UserDeletionActor{Kind: "self"}); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
}
