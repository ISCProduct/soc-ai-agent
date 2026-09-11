package application

import (
	"errors"
	"testing"

	"Backend/internal/services/shared"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func expectIsAdmin(mock sqlmock.Sqlmock, userID uint, isAdmin bool) {
	mock.ExpectQuery("SELECT .*is_admin.* FROM `users` WHERE id = \\?").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"is_admin"}).AddRow(isAdmin))
}

func expectOwnsCompany(mock sqlmock.Sqlmock, userID, companyID uint, n int64) {
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `company_ownerships` WHERE user_id = \\? AND company_id = \\?").
		WithArgs(userID, companyID).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(n))
}

func TestGetCorrelation_NonAdminOmitCompanyIDForbidden(t *testing.T) {
	db, mock := newSQLMockDB(t)
	expectIsAdmin(mock, 1, false)
	s := &ApplicationService{db: db}
	_, err := s.GetCorrelation(1, 0)
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("got %v want ErrForbidden", err)
	}
}

func TestGetCorrelation_OtherCompanyForbidden(t *testing.T) {
	db, mock := newSQLMockDB(t)
	expectIsAdmin(mock, 1, false)
	expectOwnsCompany(mock, 1, 99, 0)
	s := &ApplicationService{db: db}
	_, err := s.GetCorrelation(1, 99)
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("got %v want ErrForbidden", err)
	}
}

func TestListForOwner_OtherCompanyForbidden(t *testing.T) {
	db, mock := newSQLMockDB(t)
	expectOwnsCompany(mock, 1, 99, 0)
	s := &ApplicationService{db: db}
	_, err := s.ListForOwner(1, 99, "")
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("got %v want ErrForbidden", err)
	}
}

func TestListForOwner_MissingCompanyIDForbidden(t *testing.T) {
	s := &ApplicationService{}
	_, err := s.ListForOwner(1, 0, "")
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("got %v want ErrForbidden", err)
	}
}

func TestRequireCompanyOwner_OwnerOK(t *testing.T) {
	db, mock := newSQLMockDB(t)
	expectOwnsCompany(mock, 1, 10, 1)
	s := &ApplicationService{db: db}
	if err := s.requireCompanyOwner(1, 10); err != nil {
		t.Fatalf("got %v want nil", err)
	}
}
