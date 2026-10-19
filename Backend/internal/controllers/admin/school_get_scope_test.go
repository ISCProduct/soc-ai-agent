package admin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	admincontrollers "Backend/internal/controllers/admin"
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"
	"Backend/internal/services/school"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/mock"
)

// Get / ListCompanyApprovals は担当校スコープで保護される（#職員ロールのレビュー指摘）。
// EchoAdminAuth を職員へ開放した後、これらが無検証だと担当校外の学校情報・承認企業一覧を
// 任意の :id で読めてしまう。

func newSchoolGetCtx(schoolID string) (echo.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/schools/"+schoolID, nil), 1)
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues(schoolID)
	return ctx, rec
}

func TestAdminSchoolController_Get_OtherSchoolDenied(t *testing.T) {
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("ListSchoolsForAdmin", uint(1)).Return([]models.School{{ID: 5, Name: "担当校"}}, nil)
	ctx, _ := newSchoolGetCtx("9")
	ctrl := admincontrollers.NewAdminSchoolController(school.NewSchoolService(repo))
	testsupport.AssertStatus(t, ctrl.Get, ctx, http.StatusForbidden)
	repo.AssertNotCalled(t, "FindByID", mock.Anything)
}

func TestAdminSchoolController_Get_OwnSchoolAllowed(t *testing.T) {
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("ListSchoolsForAdmin", uint(1)).Return([]models.School{{ID: 5, Name: "担当校"}}, nil)
	repo.On("FindByID", uint(5)).Return(&models.School{ID: 5, Name: "担当校"}, nil)
	ctx, _ := newSchoolGetCtx("5")
	ctrl := admincontrollers.NewAdminSchoolController(school.NewSchoolService(repo))
	testsupport.AssertStatus(t, ctrl.Get, ctx, http.StatusOK)
}

func TestAdminSchoolController_ListCompanyApprovals_OtherSchoolDenied(t *testing.T) {
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("ListSchoolsForAdmin", uint(1)).Return([]models.School{{ID: 5, Name: "担当校"}}, nil)
	rec := httptest.NewRecorder()
	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/schools/9/company-approvals", nil), 1)
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("9")
	ctrl := admincontrollers.NewAdminSchoolController(school.NewSchoolService(repo))
	testsupport.AssertStatus(t, ctrl.ListCompanyApprovals, ctx, http.StatusForbidden)
	repo.AssertNotCalled(t, "ListApprovedCompanyIDs", mock.Anything)
}
