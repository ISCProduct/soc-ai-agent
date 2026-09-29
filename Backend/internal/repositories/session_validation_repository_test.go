package repositories

import (
	"errors"
	"testing"

	"Backend/internal/services/shared"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newSessionValidationRepositoryTestDB(t *testing.T) (*SessionValidationRepository, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock 作成失敗: %v", err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm open 失敗: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	return NewSessionValidationRepository(db), mock
}

func expectSessionValidationLookup(mock sqlmock.Sqlmock, sessionID string, row *sqlmock.Rows) {
	mock.ExpectQuery("SELECT .* FROM `session_validations`").
		WithArgs(sessionID, 1).
		WillReturnRows(row)
}

func expectOtherUserMessageCount(mock sqlmock.Sqlmock, sessionID string, userID uint, count int64) {
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `chat_messages`").
		WithArgs(sessionID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(count))
}

func TestClaimSessionOwnership_UnsetOwnerWithOtherUserMessageIsForbidden(t *testing.T) {
	repo, mock := newSessionValidationRepositoryTestDB(t)
	const sessionID = "legacy-owned-by-other"
	const requestingUserID = uint(22)

	mock.ExpectBegin()
	expectSessionValidationLookup(mock, sessionID, sqlmock.NewRows([]string{
		"id", "session_id", "user_id", "invalid_answer_count", "is_terminated",
		"last_invalid_answer_time", "created_at", "updated_at",
	}).AddRow(1, sessionID, nil, 0, false, nil, nil, nil))
	expectOtherUserMessageCount(mock, sessionID, requestingUserID, 1)
	mock.ExpectRollback()

	err := repo.ClaimSessionOwnership(sessionID, requestingUserID)
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("ClaimSessionOwnership() error = %v, want %v", err, shared.ErrForbidden)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("未充足の SQL 期待: %v", err)
	}
}

func TestClaimSessionOwnership_MissingValidationWithOtherUserMessageIsForbidden(t *testing.T) {
	repo, mock := newSessionValidationRepositoryTestDB(t)
	const sessionID = "legacy-without-validation"
	const requestingUserID = uint(22)

	mock.ExpectBegin()
	expectSessionValidationLookup(mock, sessionID, sqlmock.NewRows([]string{
		"id", "session_id", "user_id", "invalid_answer_count", "is_terminated",
		"last_invalid_answer_time", "created_at", "updated_at",
	}))
	expectOtherUserMessageCount(mock, sessionID, requestingUserID, 1)
	mock.ExpectRollback()

	err := repo.ClaimSessionOwnership(sessionID, requestingUserID)
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("ClaimSessionOwnership() error = %v, want %v", err, shared.ErrForbidden)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("未充足の SQL 期待: %v", err)
	}
}

func TestClaimSessionOwnership_NewSessionWithoutMessagesClaimsOwner(t *testing.T) {
	repo, mock := newSessionValidationRepositoryTestDB(t)
	const sessionID = "new-session"
	const firstUserID = uint(11)

	mock.ExpectBegin()
	expectSessionValidationLookup(mock, sessionID, sqlmock.NewRows([]string{
		"id", "session_id", "user_id", "invalid_answer_count", "is_terminated",
		"last_invalid_answer_time", "created_at", "updated_at",
	}))
	expectOtherUserMessageCount(mock, sessionID, firstUserID, 0)
	mock.ExpectExec("INSERT INTO `session_validations`").
		WithArgs(sessionID, 0, false, sqlmock.AnyArg(), sqlmock.AnyArg(), firstUserID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := repo.ClaimSessionOwnership(sessionID, firstUserID); err != nil {
		t.Fatalf("ClaimSessionOwnership() error = %v, want nil", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("未充足の SQL 期待: %v", err)
	}
}

func TestClaimSessionOwnership_OtherUserCannotClaimOwnedSession(t *testing.T) {
	repo, mock := newSessionValidationRepositoryTestDB(t)
	const sessionID = "owned-by-user-11"
	const requestingUserID = uint(22)
	ownerUserID := uint(11)

	mock.ExpectBegin()
	expectSessionValidationLookup(mock, sessionID, sqlmock.NewRows([]string{
		"id", "session_id", "user_id", "invalid_answer_count", "is_terminated",
		"last_invalid_answer_time", "created_at", "updated_at",
	}).AddRow(1, sessionID, ownerUserID, 0, false, nil, nil, nil))
	mock.ExpectRollback()

	err := repo.ClaimSessionOwnership(sessionID, requestingUserID)
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("ClaimSessionOwnership() error = %v, want %v", err, shared.ErrForbidden)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("未充足の SQL 期待: %v", err)
	}
}

func TestClaimSessionOwnership_OwnerCanClaimOwnedSession(t *testing.T) {
	repo, mock := newSessionValidationRepositoryTestDB(t)
	const sessionID = "owned-by-user-11"
	const ownerUserID = uint(11)

	mock.ExpectBegin()
	expectSessionValidationLookup(mock, sessionID, sqlmock.NewRows([]string{
		"id", "session_id", "user_id", "invalid_answer_count", "is_terminated",
		"last_invalid_answer_time", "created_at", "updated_at",
	}).AddRow(1, sessionID, ownerUserID, 0, false, nil, nil, nil))
	mock.ExpectCommit()

	if err := repo.ClaimSessionOwnership(sessionID, ownerUserID); err != nil {
		t.Fatalf("ClaimSessionOwnership() error = %v, want nil", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("未充足の SQL 期待: %v", err)
	}
}

func TestClaimSessionOwnership_UnsetOwnerWithOnlyRequestingUsersMessagesClaimsOwner(t *testing.T) {
	repo, mock := newSessionValidationRepositoryTestDB(t)
	const sessionID = "legacy-owned-by-requester"
	const requestingUserID = uint(22)

	mock.ExpectBegin()
	expectSessionValidationLookup(mock, sessionID, sqlmock.NewRows([]string{
		"id", "session_id", "user_id", "invalid_answer_count", "is_terminated",
		"last_invalid_answer_time", "created_at", "updated_at",
	}).AddRow(1, sessionID, nil, 0, false, nil, nil, nil))
	expectOtherUserMessageCount(mock, sessionID, requestingUserID, 0)
	mock.ExpectExec("UPDATE `session_validations`").
		WithArgs(requestingUserID, sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.ClaimSessionOwnership(sessionID, requestingUserID); err != nil {
		t.Fatalf("ClaimSessionOwnership() error = %v, want nil", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("未充足の SQL 期待: %v", err)
	}
}
