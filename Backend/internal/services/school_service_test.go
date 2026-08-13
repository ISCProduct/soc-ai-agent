package services_test

import (
	"errors"
	"testing"

	"Backend/internal/repositories"
	"Backend/internal/services"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newSchoolTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
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

func TestSchoolService_ResolveAdminAccess_Unrestricted(t *testing.T) {
	db, mock := newSchoolTestDB(t)
	svc := services.NewSchoolService(repositories.NewSchoolRepository(db))

	mock.ExpectQuery("SELECT .* FROM `schools` JOIN admin_school_memberships").
		WithArgs(uint(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "name", "status"}))

	restricted, schoolIDs, err := svc.ResolveAdminAccess(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if restricted {
		t.Fatal("expected unrestricted (system admin) for admin with no school memberships")
	}
	if len(schoolIDs) != 0 {
		t.Fatalf("expected no school IDs, got %v", schoolIDs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql: %v", err)
	}
}

func TestSchoolService_ResolveAdminAccess_Restricted(t *testing.T) {
	db, mock := newSchoolTestDB(t)
	svc := services.NewSchoolService(repositories.NewSchoolRepository(db))

	mock.ExpectQuery("SELECT .* FROM `schools` JOIN admin_school_memberships").
		WithArgs(uint(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "name", "status"}).
			AddRow(5, 2, "情報科学専門学校", "active"))

	restricted, schoolIDs, err := svc.ResolveAdminAccess(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !restricted {
		t.Fatal("expected restricted admin")
	}
	if len(schoolIDs) != 1 || schoolIDs[0] != 5 {
		t.Fatalf("schoolIDs = %v, want [5]", schoolIDs)
	}
}

func TestSchoolService_Create_Validation(t *testing.T) {
	db, _ := newSchoolTestDB(t)
	svc := services.NewSchoolService(repositories.NewSchoolRepository(db))

	if _, err := svc.Create(services.CreateSchoolInput{OrganizationID: 1, Name: ""}); !errors.Is(err, services.ErrSchoolNameRequired) {
		t.Fatalf("got %v want ErrSchoolNameRequired", err)
	}
	if _, err := svc.Create(services.CreateSchoolInput{OrganizationID: 0, Name: "情報科学専門学校"}); !errors.Is(err, services.ErrSchoolOrgRequired) {
		t.Fatalf("got %v want ErrSchoolOrgRequired", err)
	}
}
