package admin_test

// AdminCompany・AdminJobコントローラーのHTTPハンドラーテスト (Issue #430)
//
// 実行: cd Backend && go test ./internal/controllers/admin/... -run "AdminCompany|AdminJob" -v

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	admincontrollers "Backend/internal/controllers/admin"
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"

	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

// ===== AdminCompanyController =====

func newAdminCompanyController(repo *mocks.CompanyRepositoryMock, audit *mocks.AuditLogServiceMock) *admincontrollers.AdminCompanyController {
	return admincontrollers.NewAdminCompanyController(repo, audit, nil)
}

func TestAdminCompanyController_List_ServiceError(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("ListActiveFiltered", 50, 0, "", "", "", "", "", mock.Anything).Return(nil, int64(0), errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminCompanyController(repo, nil).List, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
}

func TestAdminCompanyController_List_Success(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("ListActiveFiltered", 50, 0, "", "", "", "", "", mock.Anything).Return([]models.Company{{Name: "Test Corp"}}, int64(1), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminCompanyController(repo, nil).List, testsupport.NewCtx(req, rec), http.StatusOK)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Create_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/companies", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, admincontrollers.NewAdminCompanyController(nil, nil, nil).Create, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestAdminCompanyController_Create_MissingName(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"prefecture": "東京都"})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/companies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, admincontrollers.NewAdminCompanyController(nil, nil, nil).Create, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestAdminCompanyController_Create_Success(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	repo.On("Create", mock.Anything).Return(nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	body, _ := json.Marshal(map[string]string{"name": "Test Corp"})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/companies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminCompanyController(repo, audit).Create, testsupport.NewCtx(req, rec), http.StatusOK)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Get_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies/abc", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	testsupport.AssertStatus(t, admincontrollers.NewAdminCompanyController(nil, nil, nil).Get, ctx, http.StatusBadRequest)
}

func TestAdminCompanyController_Get_NotFound(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(nil, errors.New("not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies/1", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newAdminCompanyController(repo, nil).Get, ctx, http.StatusNotFound)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Get_Success(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	company := &models.Company{Name: "Test Corp"}
	repo.On("FindByID", uint(1)).Return(company, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies/1", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newAdminCompanyController(repo, nil).Get, ctx, http.StatusOK)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Update_Success(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	company := &models.Company{Name: "Test Corp"}
	repo.On("FindByID", uint(1)).Return(company, nil)
	repo.On("Update", mock.Anything).Return(nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	body, _ := json.Marshal(map[string]string{"name": "Updated Corp"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/companies/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newAdminCompanyController(repo, audit).Update, ctx, http.StatusOK)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Publish_Success(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	company := &models.Company{Name: "Test Corp"}
	cid := uint(1)
	repo.On("FindByID", uint(1)).Return(company, nil)
	repo.On("GetWeightProfile", uint(1), (*uint)(nil)).Return(&models.CompanyWeightProfile{CompanyID: 1}, nil)
	repo.On("Update", mock.Anything).Return(nil)
	repo.On("ListJobPositions", &cid, (*uint)(nil), 1000).Return([]models.CompanyJobPosition{
		{ID: 9, CompanyID: 1, DataStatus: "draft"},
	}, nil)
	repo.On("UpdateJobPosition", mock.MatchedBy(func(p *models.CompanyJobPosition) bool {
		return p.ID == 9 && p.DataStatus == "published"
	})).Return(nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/companies/1/publish", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newAdminCompanyController(repo, audit).Publish, ctx, http.StatusOK)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Publish_RequiresWeightProfile(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	company := &models.Company{Name: "Test Corp"}
	repo.On("FindByID", uint(1)).Return(company, nil)
	repo.On("GetWeightProfile", uint(1), (*uint)(nil)).Return(nil, gorm.ErrRecordNotFound)

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/companies/1/publish", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newAdminCompanyController(repo, nil).Publish, ctx, http.StatusBadRequest)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Publish_NotFound(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(nil, errors.New("not found"))

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/companies/1/publish", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newAdminCompanyController(repo, nil).Publish, ctx, http.StatusNotFound)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Reject_Success(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	company := &models.Company{Name: "Test Corp"}
	repo.On("FindByID", uint(1)).Return(company, nil)
	repo.On("Update", mock.Anything).Return(nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/companies/1/reject", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newAdminCompanyController(repo, audit).Reject, ctx, http.StatusOK)
	repo.AssertExpectations(t)
}

// ===== AdminJobController =====

func newAdminJobController(companyRepo *mocks.CompanyRepositoryMock, jobCatRepo *mocks.JobCategoryRepositoryMock, gradRepo *mocks.GraduateEmploymentRepositoryMock, audit *mocks.AuditLogServiceMock) *admincontrollers.AdminJobController {
	return admincontrollers.NewAdminJobController(companyRepo, jobCatRepo, gradRepo, audit)
}

func TestAdminJobController_JobCategories_ServiceError(t *testing.T) {
	jobCatRepo := &mocks.JobCategoryRepositoryMock{}
	jobCatRepo.On("FindAll").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/job-categories", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminJobController(nil, jobCatRepo, nil, nil).JobCategories, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
	jobCatRepo.AssertExpectations(t)
}

func TestAdminJobController_JobCategories_Success(t *testing.T) {
	jobCatRepo := &mocks.JobCategoryRepositoryMock{}
	categories := []models.JobCategory{{Name: "Engineer"}}
	jobCatRepo.On("FindAll").Return(categories, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/job-categories", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminJobController(nil, jobCatRepo, nil, nil).JobCategories, testsupport.NewCtx(req, rec), http.StatusOK)
	jobCatRepo.AssertExpectations(t)
}

func TestAdminJobController_JobPositions_List_Success(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	positions := []models.CompanyJobPosition{{Title: "Software Engineer"}}
	companyRepo.On("ListJobPositions", (*uint)(nil), (*uint)(nil), 50).Return(positions, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/job-positions", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminJobController(companyRepo, nil, nil, nil).JobPositions, testsupport.NewCtx(req, rec), http.StatusOK)
	companyRepo.AssertExpectations(t)
}

func TestAdminJobController_GraduateEmployments_List_Success(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	entries := []models.GraduateEmployment{{GraduateName: "Test User"}}
	gradRepo.On("List", (*uint)(nil), (*uint)(nil), 50).Return(entries, nil)

	// schoolScope 適用ルートなので、絞り込み対象(ここでは絞り込みなし)を context に載せる
	req := testsupport.WithSchoolFilter(httptest.NewRequest(http.MethodGet, "/api/admin/graduate-employments", nil), nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminJobController(nil, nil, gradRepo, nil).GraduateEmployments, testsupport.NewCtx(req, rec), http.StatusOK)
	gradRepo.AssertExpectations(t)
}

func TestAdminJobController_GetGraduateEmployment_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/graduate-employments/abc", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	testsupport.AssertStatus(t, admincontrollers.NewAdminJobController(nil, nil, nil, nil).GetGraduateEmployment, ctx, http.StatusBadRequest)
}

func TestAdminJobController_GetGraduateEmployment_NotFound(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	gradRepo.On("FindByID", uint(1)).Return(nil, errors.New("not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/graduate-employments/1", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newAdminJobController(nil, nil, gradRepo, nil).GetGraduateEmployment, ctx, http.StatusNotFound)
	gradRepo.AssertExpectations(t)
}

func TestAdminJobController_GetGraduateEmployment_Success(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	entry := &models.GraduateEmployment{GraduateName: "Test User"}
	gradRepo.On("FindByID", uint(1)).Return(entry, nil)

	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/graduate-employments/1", nil), 1)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	ctrl := newAdminJobController(nil, nil, gradRepo, nil)
	ctrl.SetSchoolAccess(testsupport.NewUnrestrictedSchoolService(1))
	testsupport.AssertStatus(t, ctrl.GetGraduateEmployment, ctx, http.StatusOK)
	gradRepo.AssertExpectations(t)
}

// TestAdminJobController_GetGraduateEmployment_OtherSchoolDenied は
// 担当校を持つ管理者(先生)が他校の卒業生就職情報を単体取得できないことを検証する(#1157)。
// 一覧(GraduateEmployments)は schoolScope で絞られていたが、単体取得は素通りだった。
func TestAdminJobController_GetGraduateEmployment_OtherSchoolDenied(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	otherSchool := uint(9)
	entry := &models.GraduateEmployment{GraduateName: "他校の卒業生", SchoolID: &otherSchool}
	gradRepo.On("FindByID", uint(1)).Return(entry, nil)

	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/graduate-employments/1", nil), 1)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	ctrl := newAdminJobController(nil, nil, gradRepo, nil)
	ctrl.SetSchoolAccess(testsupport.NewRestrictedSchoolService(1, 5))
	testsupport.AssertStatus(t, ctrl.GetGraduateEmployment, ctx, http.StatusForbidden)
}

// TestAdminJobController_GetGraduateEmployment_OwnSchoolAllowed は担当校のものは取得できることを検証する。
func TestAdminJobController_GetGraduateEmployment_OwnSchoolAllowed(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	ownSchool := uint(5)
	entry := &models.GraduateEmployment{GraduateName: "自校の卒業生", SchoolID: &ownSchool}
	gradRepo.On("FindByID", uint(1)).Return(entry, nil)

	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/graduate-employments/1", nil), 1)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	ctrl := newAdminJobController(nil, nil, gradRepo, nil)
	ctrl.SetSchoolAccess(testsupport.NewRestrictedSchoolService(1, 5))
	testsupport.AssertStatus(t, ctrl.GetGraduateEmployment, ctx, http.StatusOK)
}

// TestAdminJobController_GetGraduateEmployment_SchoolAccessNotConfigured は
// SchoolService 未注入(DI漏れ)のとき fail-closed になることを検証する。
func TestAdminJobController_GetGraduateEmployment_SchoolAccessNotConfigured(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	gradRepo.On("FindByID", uint(1)).Return(&models.GraduateEmployment{}, nil)

	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/graduate-employments/1", nil), 1)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newAdminJobController(nil, nil, gradRepo, nil).GetGraduateEmployment, ctx, http.StatusInternalServerError)
}

func TestAdminJobController_UpdateGraduateEmployment_Success(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	entry := &models.GraduateEmployment{GraduateName: "Test User", CompanyID: 1}
	gradRepo.On("FindByID", uint(1)).Return(entry, nil)
	gradRepo.On("Update", mock.Anything).Return(nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	body, _ := json.Marshal(map[string]interface{}{"company_id": 1, "graduate_name": "Updated User"})
	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodPut, "/api/admin/graduate-employments/1", bytes.NewReader(body)), 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	ctrl := newAdminJobController(nil, nil, gradRepo, audit)
	ctrl.SetSchoolAccess(testsupport.NewUnrestrictedSchoolService(1))
	testsupport.AssertStatus(t, ctrl.UpdateGraduateEmployment, ctx, http.StatusOK)
	gradRepo.AssertExpectations(t)
}

// TestAdminJobController_UpdateGraduateEmployment_OtherSchoolDenied は
// 他校の卒業生就職情報を書き換えられないことを検証する(#1157)。
func TestAdminJobController_UpdateGraduateEmployment_OtherSchoolDenied(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	otherSchool := uint(9)
	gradRepo.On("FindByID", uint(1)).Return(&models.GraduateEmployment{SchoolID: &otherSchool}, nil)

	body, _ := json.Marshal(map[string]interface{}{"company_id": 1, "graduate_name": "改変"})
	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodPut, "/api/admin/graduate-employments/1", bytes.NewReader(body)), 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	ctrl := newAdminJobController(nil, nil, gradRepo, nil)
	ctrl.SetSchoolAccess(testsupport.NewRestrictedSchoolService(1, 5))
	testsupport.AssertStatus(t, ctrl.UpdateGraduateEmployment, ctx, http.StatusForbidden)
	gradRepo.AssertNotCalled(t, "Update", mock.Anything)
}

func TestAdminJobController_JobPositionAction_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/job-positions/abc/publish", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id", "action")
	ctx.SetParamValues("abc", "publish")
	testsupport.AssertStatus(t, admincontrollers.NewAdminJobController(nil, nil, nil, nil).JobPositionAction, ctx, http.StatusBadRequest)
}

func TestAdminJobController_JobPositionAction_NotFound(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindJobPositionByID", uint(1)).Return(nil, errors.New("not found"))

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/job-positions/1/publish", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id", "action")
	ctx.SetParamValues("1", "publish")
	testsupport.AssertStatus(t, newAdminJobController(companyRepo, nil, nil, nil).JobPositionAction, ctx, http.StatusNotFound)
	companyRepo.AssertExpectations(t)
}

func TestAdminJobController_JobPositionAction_UnknownAction(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	position := &models.CompanyJobPosition{Title: "Engineer"}
	companyRepo.On("FindJobPositionByID", uint(1)).Return(position, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/job-positions/1/unknown", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id", "action")
	ctx.SetParamValues("1", "unknown")
	testsupport.AssertStatus(t, newAdminJobController(companyRepo, nil, nil, nil).JobPositionAction, ctx, http.StatusBadRequest)
	companyRepo.AssertExpectations(t)
}

func TestAdminJobController_JobPositionAction_Publish_Success(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	position := &models.CompanyJobPosition{Title: "Engineer", CompanyID: 7}
	companyRepo.On("FindJobPositionByID", uint(1)).Return(position, nil)
	companyRepo.On("UpdateJobPosition", mock.Anything).Return(nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/job-positions/1/publish", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id", "action")
	ctx.SetParamValues("1", "publish")
	testsupport.AssertStatus(t, newAdminJobController(companyRepo, nil, nil, audit).JobPositionAction, ctx, http.StatusOK)
	companyRepo.AssertExpectations(t)
}

// publish は企業の公開状態を見ない。
//
// 企業が未公開のまま求人を published にしても学生側クエリで弾かれるが、
// 63031850 で「FE が確認ダイアログで警告したうえで通す」製品判断が入っている。
// サーバ側だけ 409 にすると「続行しますか？」に OK しても必ず失敗する
// 死んだ UI になるため、ここでは止めない（#1074 レビュー指摘）。
func TestAdminJobController_JobPositionAction_PublishDoesNotCheckCompany(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	position := &models.CompanyJobPosition{Title: "Engineer", CompanyID: 7}
	companyRepo.On("FindJobPositionByID", uint(1)).Return(position, nil)
	companyRepo.On("UpdateJobPosition", mock.Anything).Return(nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/job-positions/1/publish", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id", "action")
	ctx.SetParamValues("1", "publish")
	testsupport.AssertStatus(t, newAdminJobController(companyRepo, nil, nil, audit).JobPositionAction, ctx, http.StatusOK)
	companyRepo.AssertNotCalled(t, "FindByID", mock.Anything)
}

func TestAdminJobController_GraduateEmployments_Create_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/graduate-employments", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, admincontrollers.NewAdminJobController(nil, nil, nil, nil).CreateGraduateEmployment, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestAdminJobController_GraduateEmployments_Create_MissingCompanyID(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"graduate_name": "Test"})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/graduate-employments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, admincontrollers.NewAdminJobController(nil, nil, nil, nil).CreateGraduateEmployment, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestAdminJobController_GraduateEmployments_Create_Success(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	gradRepo.On("Create", mock.Anything).Return(nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	body, _ := json.Marshal(map[string]interface{}{"company_id": 1, "graduate_name": "Test User"})
	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodPost, "/api/admin/graduate-employments", bytes.NewReader(body)), 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctrl := newAdminJobController(nil, nil, gradRepo, audit)
	ctrl.SetSchoolAccess(testsupport.NewUnrestrictedSchoolService(1))
	testsupport.AssertStatus(t, ctrl.CreateGraduateEmployment, testsupport.NewCtx(req, rec), http.StatusOK)
	gradRepo.AssertExpectations(t)
}

// TestAdminJobController_CreateGraduateEmployment_StampsOwnSchool は、担当校を持つ管理者(先生)が
// 作成した卒業生就職情報に自校の school_id が入ることを検証する（#1157）。
//
// school_id が NULL のままだと、一覧(WHERE school_id = ?)にも単体取得にも現れず、
// 作成者自身が二度と参照できない行になる。
func TestAdminJobController_CreateGraduateEmployment_StampsOwnSchool(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	gradRepo.On("Create", mock.MatchedBy(func(e *models.GraduateEmployment) bool {
		return e.SchoolID != nil && *e.SchoolID == 5
	})).Return(nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	body, _ := json.Marshal(map[string]interface{}{"company_id": 1, "graduate_name": "自校の卒業生"})
	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodPost, "/api/admin/graduate-employments", bytes.NewReader(body)), 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctrl := newAdminJobController(nil, nil, gradRepo, audit)
	ctrl.SetSchoolAccess(testsupport.NewRestrictedSchoolService(1, 5))
	testsupport.AssertStatus(t, ctrl.CreateGraduateEmployment, testsupport.NewCtx(req, rec), http.StatusOK)
	gradRepo.AssertExpectations(t)
}

// TestAdminJobController_CreateGraduateEmployment_RejectsOtherSchool は、担当外の school_id を
// 指定した作成が 403 になることを検証する（#1157）。
func TestAdminJobController_CreateGraduateEmployment_RejectsOtherSchool(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}

	body, _ := json.Marshal(map[string]interface{}{"company_id": 1, "graduate_name": "他校", "school_id": 9})
	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodPost, "/api/admin/graduate-employments", bytes.NewReader(body)), 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctrl := newAdminJobController(nil, nil, gradRepo, nil)
	ctrl.SetSchoolAccess(testsupport.NewRestrictedSchoolService(1, 5))
	testsupport.AssertStatus(t, ctrl.CreateGraduateEmployment, testsupport.NewCtx(req, rec), http.StatusForbidden)
	gradRepo.AssertNotCalled(t, "Create", mock.Anything)
}

// TestAdminJobController_CreateJobPosition_InheritsCompanyStatus は、
// 新規求人の公開状態が所属企業に追随することを検証する。
//
// DBデフォルトの draft のままだと、公開済み企業に管理者が求人を足しても
// 学生側のクエリ(data_status='published')に乗らず、別途 publish が必要になる（#1074）。
func TestAdminJobController_CreateJobPosition_InheritsCompanyStatus(t *testing.T) {
	tests := []struct {
		name          string
		companyStatus string
		wantStatus    string
	}{
		{name: "公開済み企業の求人は published で作られる", companyStatus: "published", wantStatus: "published"},
		{name: "未公開企業の求人は draft のまま", companyStatus: "draft", wantStatus: "draft"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			companyRepo := &mocks.CompanyRepositoryMock{}
			audit := &mocks.AuditLogServiceMock{}
			companyRepo.On("FindByID", uint(7)).
				Return(&models.Company{ID: 7, DataStatus: tt.companyStatus}, nil)

			var created *models.CompanyJobPosition
			companyRepo.On("CreateJobPosition", mock.Anything).
				Run(func(args mock.Arguments) {
					created = args.Get(0).(*models.CompanyJobPosition)
				}).Return(nil)
			audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

			body, _ := json.Marshal(map[string]any{
				"company_id": 7, "title": "Engineer", "job_category_id": 3,
			})
			req := httptest.NewRequest(http.MethodPost, "/api/admin/job-positions", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			testsupport.AssertStatus(t, newAdminJobController(companyRepo, nil, nil, audit).CreateJobPosition, testsupport.NewCtx(req, rec), http.StatusOK)

			if created == nil {
				t.Fatal("CreateJobPosition が呼ばれていない")
			}
			if created.DataStatus != tt.wantStatus {
				t.Errorf("DataStatus = %q, want %q", created.DataStatus, tt.wantStatus)
			}
		})
	}
}

// CreateJobPosition は企業を引けなければ作成しない。
//
// エラーを握り潰して継承をスキップすると DB デフォルトの draft で作られ、
// 「公開済み企業に足したのに学生に出ない」が無言で再発する（#1074 レビュー指摘 S5）。
// 存在しない company_id の求人が宙に浮くのも防ぐ。
func TestAdminJobController_CreateJobPosition_RejectsUnknownCompany(t *testing.T) {
	for _, tt := range []struct {
		name    string
		company *models.Company
		err     error
	}{
		{name: "企業が存在しない", company: nil, err: errors.New("record not found")},
		{name: "DB障害", company: nil, err: errors.New("connection refused")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			companyRepo := &mocks.CompanyRepositoryMock{}
			companyRepo.On("FindByID", uint(7)).Return(tt.company, tt.err)

			body, _ := json.Marshal(map[string]any{
				"company_id": 7, "title": "Engineer", "job_category_id": 3,
			})
			req := httptest.NewRequest(http.MethodPost, "/api/admin/job-positions", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			testsupport.AssertStatus(t, newAdminJobController(companyRepo, nil, nil, nil).CreateJobPosition, testsupport.NewCtx(req, rec), http.StatusBadRequest)
			// 作成に進まないこと。
			companyRepo.AssertNotCalled(t, "CreateJobPosition", mock.Anything)
		})
	}
}
