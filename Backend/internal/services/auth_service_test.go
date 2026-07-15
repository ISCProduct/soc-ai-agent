package services

import (
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newAuthServiceTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
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
	return db, mock
}

func TestDeleteAccount_DBNotConfigured(t *testing.T) {
	svc := &AuthService{}
	if err := svc.DeleteAccount(1); err == nil {
		t.Fatal("expected error when db is not configured")
	}
}

func TestDeleteAccount_SoftWithdraw(t *testing.T) {
	db, mock := newAuthServiceTestDB(t)
	svc := &AuthService{db: db}
	svc.rebuildDeletionService()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE `users`.`id` = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs(uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "withdrawn_at"}).AddRow(1, "user@example.com", nil))
	mock.ExpectQuery("SELECT \\* FROM `interview_videos` WHERE user_id = \\?").
		WithArgs(uint(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "drive_file_id"}))
	mock.ExpectQuery("SELECT \\* FROM `resume_documents` WHERE user_id = \\?").
		WithArgs(uint(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "stored_path"}))
	mock.ExpectExec("INSERT INTO `withdrawn_users`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE `users` SET").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := svc.DeleteAccount(1); err != nil {
		t.Fatalf("DeleteAccount returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestDeleteAccount_AlreadyWithdrawn(t *testing.T) {
	db, mock := newAuthServiceTestDB(t)
	svc := &AuthService{db: db}
	svc.rebuildDeletionService()

	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE `users`.`id` = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs(uint(2), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "withdrawn_at"}).AddRow(2, "withdrawn+2@deleted.invalid", now))
	mock.ExpectRollback()

	if err := svc.DeleteAccount(2); err != ErrAlreadyWithdrawn {
		t.Fatalf("expected ErrAlreadyWithdrawn, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}
