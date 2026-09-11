package organization_test

import (
	"errors"
	"testing"

	"Backend/internal/repositories"
	"Backend/internal/services/organization"

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

func TestOrganizationService_InterviewAccess(t *testing.T) {
	tests := []struct {
		name    string
		orgID   uint
		sessID  uint
		rows    bool
		wantErr error
	}{
		{name: "cross org denied", orgID: 1, sessID: 99, rows: false, wantErr: organization.ErrCrossOrganization},
		{name: "same org allowed", orgID: 2, sessID: 10, rows: true, wantErr: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newOrgTestDB(t)
			repo := repositories.NewOrganizationRepository(db)
			svc := organization.NewOrganizationService(repo)

			q := mock.ExpectQuery("SELECT \\* FROM `interview_sessions` WHERE id = \\? AND organization_id = \\? ORDER BY `interview_sessions`.`id` LIMIT \\?").
				WithArgs(tt.sessID, tt.orgID, 1)
			cols := sqlmock.NewRows([]string{"id", "organization_id", "user_id"})
			if tt.rows {
				cols = cols.AddRow(tt.sessID, tt.orgID, 5)
			}
			q.WillReturnRows(cols)

			_, err := svc.GetInterviewSessionForOrganization(tt.orgID, tt.sessID)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v want %v", err, tt.wantErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("sql: %v", err)
			}
		})
	}
}

func TestOrganizationService_CrossOrgResumeAccessDenied(t *testing.T) {
	db, mock := newOrgTestDB(t)
	repo := repositories.NewOrganizationRepository(db)
	svc := organization.NewOrganizationService(repo)

	mock.ExpectQuery("SELECT \\* FROM `resume_documents` WHERE id = \\? AND organization_id = \\? ORDER BY `resume_documents`.`id` LIMIT \\?").
		WithArgs(uint(7), uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "user_id"}))

	_, err := svc.GetResumeDocumentForOrganization(1, 7)
	if !errors.Is(err, organization.ErrCrossOrganization) {
		t.Fatalf("expected ErrCrossOrganization, got %v", err)
	}
}

func TestOrganizationService_CrossOrgChatAccessDenied(t *testing.T) {
	db, mock := newOrgTestDB(t)
	repo := repositories.NewOrganizationRepository(db)
	svc := organization.NewOrganizationService(repo)

	mock.ExpectQuery("SELECT \\* FROM `chat_messages` WHERE id = \\? AND organization_id = \\? ORDER BY `chat_messages`.`id` LIMIT \\?").
		WithArgs(uint(3), uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "user_id"}))

	_, err := svc.GetChatMessageForOrganization(1, 3)
	if !errors.Is(err, organization.ErrCrossOrganization) {
		t.Fatalf("expected ErrCrossOrganization, got %v", err)
	}
}

func TestOrganizationService_CreateRejectsBadSlug(t *testing.T) {
	db, mock := newOrgTestDB(t)
	repo := repositories.NewOrganizationRepository(db)
	svc := organization.NewOrganizationService(repo)

	_, err := svc.Create(organization.CreateOrganizationInput{Name: "Acme", Slug: "-bad"})
	if !errors.Is(err, organization.ErrInvalidOrgSlug) {
		t.Fatalf("expected ErrInvalidOrgSlug, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("no SQL expected: %v", err)
	}
}
