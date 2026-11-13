package application

import (
	"errors"
	"testing"

	"Backend/internal/services/shared"
)

// stubAccess は shared.CompanyAccessReader の偽物。問い合わせの回数も数える。
type stubAccess struct {
	isAdmin     bool
	owns        bool
	adminCalls  int
	ownsCalls   int
	lastOwnsFor [2]uint // (userID, companyID)
}

func (a *stubAccess) IsAdmin(uint) (bool, error) { a.adminCalls++; return a.isAdmin, nil }
func (a *stubAccess) OwnsCompany(user, company uint) (bool, error) {
	a.ownsCalls++
	a.lastOwnsFor = [2]uint{user, company}
	return a.owns, nil
}

func TestGetCorrelation_NonAdminOmitCompanyIDForbidden(t *testing.T) {
	access := &stubAccess{}
	s := &ApplicationService{access: access}
	_, err := s.GetCorrelation(1, 0)
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("got %v want ErrForbidden", err)
	}
	if access.adminCalls != 1 {
		t.Fatalf("管理者かどうかを1回確かめるはず: %d", access.adminCalls)
	}
}

func TestGetCorrelation_OtherCompanyForbidden(t *testing.T) {
	access := &stubAccess{}
	s := &ApplicationService{access: access}
	_, err := s.GetCorrelation(1, 99)
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("got %v want ErrForbidden", err)
	}
	if access.ownsCalls != 1 || access.lastOwnsFor != [2]uint{1, 99} {
		t.Fatalf("所有者の確認 = %d回 %v, want 1回 [1 99]", access.ownsCalls, access.lastOwnsFor)
	}
}

func TestListForOwner_OtherCompanyForbidden(t *testing.T) {
	access := &stubAccess{}
	s := &ApplicationService{access: access}
	_, err := s.ListForOwner(1, 99, "")
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("got %v want ErrForbidden", err)
	}
	if access.ownsCalls != 1 || access.lastOwnsFor != [2]uint{1, 99} {
		t.Fatalf("所有者の確認 = %d回 %v, want 1回 [1 99]", access.ownsCalls, access.lastOwnsFor)
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
	s := &ApplicationService{access: &stubAccess{owns: true}}
	if err := s.requireCompanyOwner(1, 10); err != nil {
		t.Fatalf("got %v want nil", err)
	}
}

// access が無いときは、所有者でも管理者でもないものとして拒否する(fail-closed)。
func TestRequireCompanyOwner_NoAccessReaderIsForbidden(t *testing.T) {
	s := &ApplicationService{}
	if err := s.requireCompanyOwner(1, 10); !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("got %v want ErrForbidden", err)
	}
}
