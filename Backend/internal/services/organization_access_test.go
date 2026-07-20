package services_test

import (
	"testing"

	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newOrgTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
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

func TestOrganizationService_CrossOrgInterviewAccessDenied(t *testing.T) {
	db, mock := newOrgTestDB(t)
	repo := repositories.NewOrganizationRepository(db)
	svc := services.NewOrganizationService(repo)

	// org 1 から org 2 の面接セッションを参照 → 見つからない
	mock.ExpectQuery("SELECT \\* FROM `interview_sessions` WHERE id = \\? AND organization_id = \\? ORDER BY `interview_sessions`.`id` LIMIT \\?").
		WithArgs(uint(99), uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "user_id"}))

	_, err := svc.GetInterviewSessionForOrganization(1, 99)
	if err != services.ErrCrossOrganization {
		t.Fatalf("expected ErrCrossOrganization, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}
}

func TestOrganizationService_SameOrgInterviewAccessAllowed(t *testing.T) {
	db, mock := newOrgTestDB(t)
	repo := repositories.NewOrganizationRepository(db)
	svc := services.NewOrganizationService(repo)

	mock.ExpectQuery("SELECT \\* FROM `interview_sessions` WHERE id = \\? AND organization_id = \\? ORDER BY `interview_sessions`.`id` LIMIT \\?").
		WithArgs(uint(10), uint(2), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "user_id"}).
			AddRow(10, 2, 5))

	session, err := svc.GetInterviewSessionForOrganization(2, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session == nil || session.OrganizationID != 2 {
		t.Fatalf("unexpected session: %+v", session)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}
}

func TestOrganizationService_CrossOrgResumeAccessDenied(t *testing.T) {
	db, mock := newOrgTestDB(t)
	repo := repositories.NewOrganizationRepository(db)
	svc := services.NewOrganizationService(repo)

	mock.ExpectQuery("SELECT \\* FROM `resume_documents` WHERE id = \\? AND organization_id = \\? ORDER BY `resume_documents`.`id` LIMIT \\?").
		WithArgs(uint(7), uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "user_id"}))

	_, err := svc.GetResumeDocumentForOrganization(1, 7)
	if err != services.ErrCrossOrganization {
		t.Fatalf("expected ErrCrossOrganization, got %v", err)
	}
}

func TestOrganizationService_CrossOrgChatAccessDenied(t *testing.T) {
	db, mock := newOrgTestDB(t)
	repo := repositories.NewOrganizationRepository(db)
	svc := services.NewOrganizationService(repo)

	mock.ExpectQuery("SELECT \\* FROM `chat_messages` WHERE id = \\? AND organization_id = \\? ORDER BY `chat_messages`.`id` LIMIT \\?").
		WithArgs(uint(3), uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "user_id"}))

	_, err := svc.GetChatMessageForOrganization(1, 3)
	if err != services.ErrCrossOrganization {
		t.Fatalf("expected ErrCrossOrganization, got %v", err)
	}
}

func TestOrganizationService_CreateRejectsBadSlug(t *testing.T) {
	db, mock := newOrgTestDB(t)
	repo := repositories.NewOrganizationRepository(db)
	svc := services.NewOrganizationService(repo)

	_, err := svc.Create(services.CreateOrganizationInput{Name: "Acme", Slug: "-bad"})
	if err != services.ErrInvalidOrgSlug {
		t.Fatalf("expected ErrInvalidOrgSlug, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("no SQL expected: %v", err)
	}
	_ = models.DefaultOrganizationID
}
