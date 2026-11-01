package admin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/controllers/httpapi"
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"
	"Backend/internal/services/school"

	"github.com/labstack/echo/v4"
)

// 1件ごとの認可判定は、対象ユーザーの school_id をそのまま信用してはいけない（#1675）。
// school_id はプロフィールの学校名（自己申告）から解決されるため、学校が本人の学園に
// 属していなければ未所属として扱う。担当校ありの管理者・職員には見せない。
func TestEnsureAdminSchoolAccessForUser_RejectsSchoolOutsideTargetOrganization(t *testing.T) {
	schoolID := uint(5)
	tests := []struct {
		name         string
		targetOrgID  uint
		schoolOrgID  uint
		wantHTTPCode int
	}{
		// 担当校(5)に属していても、学校の学園と本人の学園が違えば通してはいけない。
		{"学園が一致しない", 2, 1, http.StatusForbidden},
		{"学園が一致する", 1, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mocks.SchoolRepositoryMock{}
			// 担当校ありの職員（restricted）
			repo.On("ListSchoolsForAdmin", uint(42)).
				Return([]models.School{{ID: schoolID, Name: "担当校"}}, nil)
			repo.On("FindByID", schoolID).
				Return(&models.School{ID: schoolID, OrganizationID: tt.schoolOrgID}, nil)

			req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/", nil), 42)
			ctx := testsupport.NewCtx(req, httptest.NewRecorder())

			err := httpapi.EnsureAdminSchoolAccessForUser(ctx, school.NewSchoolService(repo), tt.targetOrgID, &schoolID)
			if tt.wantHTTPCode == 0 {
				if err != nil {
					t.Fatalf("同一学園なら通すべき: %v", err)
				}
				return
			}
			he, ok := err.(*echo.HTTPError)
			if !ok || he.Code != tt.wantHTTPCode {
				t.Fatalf("学園不一致を拒否していない: err=%v", err)
			}
		})
	}
}

// 無制限の管理者は学校で絞らないため、学園一致の判定に入らず学校も引き直さない。
// 引き直すと余分な問い合わせが増えるうえ、FindByID のスタブが無い既存テストが壊れる。
func TestEnsureAdminSchoolAccessForUser_UnrestrictedAdminSkipsLookup(t *testing.T) {
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("ListSchoolsForAdmin", uint(42)).Return([]models.School{}, nil)

	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/", nil), 42)
	ctx := testsupport.NewCtx(req, httptest.NewRecorder())

	schoolID := uint(5)
	if err := httpapi.EnsureAdminSchoolAccessForUser(ctx, school.NewSchoolService(repo), 2, &schoolID); err != nil {
		t.Fatalf("無制限管理者は通すべき: %v", err)
	}
	repo.AssertNotCalled(t, "FindByID", schoolID)
}

// 担当校は学園をまたげない（#1675）。またげると、他学園の職員に担当校を割り当てるだけで
// その学校の学生データへ到達させられる。
func TestSchoolService_AddMemberRejectsCrossOrganization(t *testing.T) {
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("FindByID", uint(5)).Return(&models.School{ID: 5, OrganizationID: 1}, nil)

	svc := school.NewSchoolService(repo, stubUserReader{user: &entity.User{ID: 2, OrganizationID: 9}})
	if err := svc.AddMember(2, 5); err != school.ErrMemberOrganizationMismatch {
		t.Fatalf("学園をまたいだ割当を許している: %v", err)
	}
	repo.AssertNotCalled(t, "AddMember")
}

// ユーザー参照を渡さない構成では割当を通さない（fail-close）。
// 検査を飛ばして割当が通る方が危険なので、未注入はエラーにする。
func TestSchoolService_AddMemberFailsClosedWithoutUserReader(t *testing.T) {
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("FindByID", uint(5)).Return(&models.School{ID: 5, OrganizationID: 1}, nil)

	if err := school.NewSchoolService(repo).AddMember(2, 5); err == nil {
		t.Fatal("ユーザー参照が無いのに割当が通っている")
	}
	repo.AssertNotCalled(t, "AddMember")
}
