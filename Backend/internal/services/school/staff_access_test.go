package school_test

import (
	"testing"

	"Backend/internal/repositories"
	"Backend/internal/services/school"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// ResolveAccess の肝: 「無制限（全校閲覧）」は is_admin のときだけ。
// 職員（is_admin=false）は担当校0件でも restricted=true（allowed空＝何も見えない）。
// これが崩れると担当校未設定の職員が全校の学生PIIを見られる。

func expectMemberships(mock sqlmock.Sqlmock, userID uint, schoolIDs ...uint) {
	rows := sqlmock.NewRows([]string{"id", "organization_id", "name", "status"})
	for _, id := range schoolIDs {
		rows.AddRow(id, 1, "x", "active")
	}
	mock.ExpectQuery("SELECT .* FROM `schools` JOIN admin_school_memberships").
		WithArgs(userID).WillReturnRows(rows)
}

func TestResolveAccess_PlatformAdminNoMemberships_Unrestricted(t *testing.T) {
	db, mock := newSchoolTestDB(t)
	svc := school.NewSchoolService(repositories.NewSchoolRepository(db))
	expectMemberships(mock, 1) // 0件
	restricted, ids, err := svc.ResolveAccess(true, 1)
	if err != nil {
		t.Fatal(err)
	}
	if restricted || len(ids) != 0 {
		t.Fatalf("platform admin with no memberships must be unrestricted; got restricted=%v ids=%v", restricted, ids)
	}
}

func TestResolveAccess_StaffNoMemberships_RestrictedEmpty(t *testing.T) {
	db, mock := newSchoolTestDB(t)
	svc := school.NewSchoolService(repositories.NewSchoolRepository(db))
	expectMemberships(mock, 2) // 0件
	restricted, ids, err := svc.ResolveAccess(false, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !restricted || len(ids) != 0 {
		t.Fatalf("non-admin staff with no memberships must be restricted-empty; got restricted=%v ids=%v", restricted, ids)
	}
}

func TestResolveAccess_StaffWithMemberships_RestrictedToThem(t *testing.T) {
	db, mock := newSchoolTestDB(t)
	svc := school.NewSchoolService(repositories.NewSchoolRepository(db))
	expectMemberships(mock, 3, 5)
	restricted, ids, err := svc.ResolveAccess(false, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !restricted || len(ids) != 1 || ids[0] != 5 {
		t.Fatalf("staff must be scoped to its schools; got restricted=%v ids=%v", restricted, ids)
	}
}

func TestCanAdminAccessSchool_StaffNoMemberships_Denied(t *testing.T) {
	db, mock := newSchoolTestDB(t)
	svc := school.NewSchoolService(repositories.NewSchoolRepository(db))
	expectMemberships(mock, 2)
	target := uint(5)
	allowed, err := svc.CanAdminAccessSchool(false, 2, &target)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("non-admin staff with no memberships must be denied any school")
	}
}

func TestCanAdminAccessSchool_PlatformAdminNoMemberships_Allowed(t *testing.T) {
	db, mock := newSchoolTestDB(t)
	svc := school.NewSchoolService(repositories.NewSchoolRepository(db))
	expectMemberships(mock, 1)
	target := uint(5)
	allowed, err := svc.CanAdminAccessSchool(true, 1, &target)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("platform admin must access any school")
	}
}
