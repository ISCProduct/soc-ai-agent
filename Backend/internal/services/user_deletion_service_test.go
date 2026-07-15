package services

import (
	"context"
	"sync"
	"testing"
	"time"

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

func TestUserDeletionService_SoftWithdrawDoesNotDeleteS3(t *testing.T) {
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
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "withdrawn_at"}).AddRow(9, "del@example.com", nil))
	mock.ExpectQuery("SELECT \\* FROM `interview_videos` WHERE user_id = \\?").
		WithArgs(uint(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "drive_file_id"}).
			AddRow(1, "interview-videos/1/a.webm"))
	mock.ExpectQuery("SELECT \\* FROM `resume_documents` WHERE user_id = \\?").
		WithArgs(uint(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "stored_path", "normalized_path", "annotated_path"}).
			AddRow(2, "s3://bucket/resumes/9/doc.pdf", "", ""))
	mock.ExpectExec("INSERT INTO `withdrawn_users`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE `users` SET").
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
	if len(obj.keys) != 0 {
		t.Fatalf("soft withdraw must not delete S3 immediately, got %v", obj.keys)
	}
}

func TestUserDeletionService_EnsureActiveUser(t *testing.T) {
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

	svc := NewUserDeletionService(db, nil, nil)
	now := time.Now().UTC()
	mock.ExpectQuery("SELECT `id`,`withdrawn_at` FROM `users` WHERE `users`.`id` = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs(uint(3), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "withdrawn_at"}).AddRow(3, now))

	if err := svc.EnsureActiveUser(3); err != ErrAccountWithdrawn {
		t.Fatalf("expected ErrAccountWithdrawn, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}
}
