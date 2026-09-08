package controllers_test

// AdminCompany・AdminJobコントローラーのHTTPハンドラーテスト (Issue #430)
//
// 実行: cd Backend && go test ./test/controllers/... -run "AdminCompany|AdminJob" -v

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	"Backend/internal/models"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

// ===== AdminCompanyController =====

func newAdminCompanyController(repo *mocks.CompanyRepositoryMock, audit *mocks.AuditLogServiceMock) *controllers.AdminCompanyController {
	return controllers.NewAdminCompanyController(repo, audit, nil)
}

func TestAdminCompanyController_List_ServiceError(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("ListActiveFiltered", 50, 0, "", "", "", "", "", mock.Anything).Return(nil, int64(0), errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newAdminCompanyController(repo, nil).List, newCtx(req, rec), http.StatusInternalServerError)
}

func TestAdminCompanyController_List_Success(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("ListActiveFiltered", 50, 0, "", "", "", "", "", mock.Anything).Return([]models.Company{{Name: "Test Corp"}}, int64(1), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newAdminCompanyController(repo, nil).List, newCtx(req, rec), http.StatusOK)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Create_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/companies", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminCompanyController(nil, nil, nil).Create, newCtx(req, rec), http.StatusBadRequest)
}

func TestAdminCompanyController_Create_MissingName(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"prefecture": "東京都"})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/companies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminCompanyController(nil, nil, nil).Create, newCtx(req, rec), http.StatusBadRequest)
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
	assertStatus(t, newAdminCompanyController(repo, audit).Create, newCtx(req, rec), http.StatusOK)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Get_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies/abc", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	assertStatus(t, controllers.NewAdminCompanyController(nil, nil, nil).Get, ctx, http.StatusBadRequest)
}

func TestAdminCompanyController_Get_NotFound(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(nil, errors.New("not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies/1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newAdminCompanyController(repo, nil).Get, ctx, http.StatusNotFound)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Get_Success(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	company := &models.Company{Name: "Test Corp"}
	repo.On("FindByID", uint(1)).Return(company, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies/1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newAdminCompanyController(repo, nil).Get, ctx, http.StatusOK)
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
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newAdminCompanyController(repo, audit).Update, ctx, http.StatusOK)
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
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newAdminCompanyController(repo, audit).Publish, ctx, http.StatusOK)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Publish_RequiresWeightProfile(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	company := &models.Company{Name: "Test Corp"}
	repo.On("FindByID", uint(1)).Return(company, nil)
	repo.On("GetWeightProfile", uint(1), (*uint)(nil)).Return(nil, gorm.ErrRecordNotFound)

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/companies/1/publish", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newAdminCompanyController(repo, nil).Publish, ctx, http.StatusBadRequest)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Publish_NotFound(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(nil, errors.New("not found"))

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/companies/1/publish", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newAdminCompanyController(repo, nil).Publish, ctx, http.StatusNotFound)
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
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newAdminCompanyController(repo, audit).Reject, ctx, http.StatusOK)
	repo.AssertExpectations(t)
}

// ===== AdminJobController =====

func newAdminJobController(companyRepo *mocks.CompanyRepositoryMock, jobCatRepo *mocks.JobCategoryRepositoryMock, gradRepo *mocks.GraduateEmploymentRepositoryMock, audit *mocks.AuditLogServiceMock) *controllers.AdminJobController {
	return controllers.NewAdminJobController(companyRepo, jobCatRepo, gradRepo, audit)
}

func TestAdminJobController_JobCategories_ServiceError(t *testing.T) {
	jobCatRepo := &mocks.JobCategoryRepositoryMock{}
	jobCatRepo.On("FindAll").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/job-categories", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newAdminJobController(nil, jobCatRepo, nil, nil).JobCategories, newCtx(req, rec), http.StatusInternalServerError)
	jobCatRepo.AssertExpectations(t)
}

func TestAdminJobController_JobCategories_Success(t *testing.T) {
	jobCatRepo := &mocks.JobCategoryRepositoryMock{}
	categories := []models.JobCategory{{Name: "Engineer"}}
	jobCatRepo.On("FindAll").Return(categories, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/job-categories", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newAdminJobController(nil, jobCatRepo, nil, nil).JobCategories, newCtx(req, rec), http.StatusOK)
	jobCatRepo.AssertExpectations(t)
}

func TestAdminJobController_JobPositions_List_Success(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	positions := []models.CompanyJobPosition{{Title: "Software Engineer"}}
	companyRepo.On("ListJobPositions", (*uint)(nil), (*uint)(nil), 50).Return(positions, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/job-positions", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newAdminJobController(companyRepo, nil, nil, nil).JobPositions, newCtx(req, rec), http.StatusOK)
	companyRepo.AssertExpectations(t)
}

func TestAdminJobController_GraduateEmployments_List_Success(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	entries := []models.GraduateEmployment{{GraduateName: "Test User"}}
	gradRepo.On("List", (*uint)(nil), (*uint)(nil), 50).Return(entries, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/graduate-employments", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newAdminJobController(nil, nil, gradRepo, nil).GraduateEmployments, newCtx(req, rec), http.StatusOK)
	gradRepo.AssertExpectations(t)
}

func TestAdminJobController_GetGraduateEmployment_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/graduate-employments/abc", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	assertStatus(t, controllers.NewAdminJobController(nil, nil, nil, nil).GetGraduateEmployment, ctx, http.StatusBadRequest)
}

func TestAdminJobController_GetGraduateEmployment_NotFound(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	gradRepo.On("FindByID", uint(1)).Return(nil, errors.New("not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/graduate-employments/1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newAdminJobController(nil, nil, gradRepo, nil).GetGraduateEmployment, ctx, http.StatusNotFound)
	gradRepo.AssertExpectations(t)
}

func TestAdminJobController_GetGraduateEmployment_Success(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	entry := &models.GraduateEmployment{GraduateName: "Test User"}
	gradRepo.On("FindByID", uint(1)).Return(entry, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/graduate-employments/1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newAdminJobController(nil, nil, gradRepo, nil).GetGraduateEmployment, ctx, http.StatusOK)
	gradRepo.AssertExpectations(t)
}

func TestAdminJobController_UpdateGraduateEmployment_Success(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	entry := &models.GraduateEmployment{GraduateName: "Test User", CompanyID: 1}
	gradRepo.On("FindByID", uint(1)).Return(entry, nil)
	gradRepo.On("Update", mock.Anything).Return(nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	body, _ := json.Marshal(map[string]interface{}{"company_id": 1, "graduate_name": "Updated User"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/graduate-employments/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newAdminJobController(nil, nil, gradRepo, audit).UpdateGraduateEmployment, ctx, http.StatusOK)
	gradRepo.AssertExpectations(t)
}

func TestAdminJobController_JobPositionAction_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/job-positions/abc/publish", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id", "action")
	ctx.SetParamValues("abc", "publish")
	assertStatus(t, controllers.NewAdminJobController(nil, nil, nil, nil).JobPositionAction, ctx, http.StatusBadRequest)
}

func TestAdminJobController_JobPositionAction_NotFound(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindJobPositionByID", uint(1)).Return(nil, errors.New("not found"))

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/job-positions/1/publish", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id", "action")
	ctx.SetParamValues("1", "publish")
	assertStatus(t, newAdminJobController(companyRepo, nil, nil, nil).JobPositionAction, ctx, http.StatusNotFound)
	companyRepo.AssertExpectations(t)
}

func TestAdminJobController_JobPositionAction_UnknownAction(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	position := &models.CompanyJobPosition{Title: "Engineer"}
	companyRepo.On("FindJobPositionByID", uint(1)).Return(position, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/job-positions/1/unknown", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id", "action")
	ctx.SetParamValues("1", "unknown")
	assertStatus(t, newAdminJobController(companyRepo, nil, nil, nil).JobPositionAction, ctx, http.StatusBadRequest)
	companyRepo.AssertExpectations(t)
}

func TestAdminJobController_JobPositionAction_Publish_Success(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	position := &models.CompanyJobPosition{Title: "Engineer", CompanyID: 7}
	companyRepo.On("FindJobPositionByID", uint(1)).Return(position, nil)
	// 企業が公開済みなら求人を公開できる。
	companyRepo.On("FindByID", uint(7)).Return(&models.Company{ID: 7, DataStatus: "published"}, nil)
	companyRepo.On("UpdateJobPosition", mock.Anything).Return(nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/job-positions/1/publish", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id", "action")
	ctx.SetParamValues("1", "publish")
	assertStatus(t, newAdminJobController(companyRepo, nil, nil, audit).JobPositionAction, ctx, http.StatusOK)
	companyRepo.AssertExpectations(t)
}

// TestAdminJobController_JobPositionAction_Publish_RejectsDraftCompany は、
// 企業が未公開のまま求人だけ公開する操作を止めることを検証する。
//
// 学生側のクエリは企業と求人の両方が published であることを要求するため、
// 企業が draft のまま求人を published にすると、どこにも出ない求人ができる。
// 「承認したのに学生に出ない」という #1074 の主訴そのもの。
func TestAdminJobController_JobPositionAction_Publish_RejectsDraftCompany(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	position := &models.CompanyJobPosition{Title: "Engineer", CompanyID: 7}
	companyRepo.On("FindJobPositionByID", uint(1)).Return(position, nil)
	companyRepo.On("FindByID", uint(7)).Return(&models.Company{ID: 7, DataStatus: "draft"}, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/job-positions/1/publish", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id", "action")
	ctx.SetParamValues("1", "publish")
	assertStatus(t, newAdminJobController(companyRepo, nil, nil, nil).JobPositionAction, ctx, http.StatusConflict)
	// 更新は一切行わないこと。
	companyRepo.AssertNotCalled(t, "UpdateJobPosition", mock.Anything)
}

// reject は企業の公開状態に関わらず通す（公開前に不採用にする運用があるため）。
func TestAdminJobController_JobPositionAction_RejectAllowedForDraftCompany(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	position := &models.CompanyJobPosition{Title: "Engineer", CompanyID: 7}
	companyRepo.On("FindJobPositionByID", uint(1)).Return(position, nil)
	companyRepo.On("UpdateJobPosition", mock.Anything).Return(nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/job-positions/1/reject", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id", "action")
	ctx.SetParamValues("1", "reject")
	assertStatus(t, newAdminJobController(companyRepo, nil, nil, audit).JobPositionAction, ctx, http.StatusOK)
	// reject では企業を引く必要が無いこと。
	companyRepo.AssertNotCalled(t, "FindByID", mock.Anything)
}

func TestAdminJobController_GraduateEmployments_Create_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/graduate-employments", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminJobController(nil, nil, nil, nil).CreateGraduateEmployment, newCtx(req, rec), http.StatusBadRequest)
}

func TestAdminJobController_GraduateEmployments_Create_MissingCompanyID(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"graduate_name": "Test"})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/graduate-employments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminJobController(nil, nil, nil, nil).CreateGraduateEmployment, newCtx(req, rec), http.StatusBadRequest)
}

func TestAdminJobController_GraduateEmployments_Create_Success(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	gradRepo.On("Create", mock.Anything).Return(nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	body, _ := json.Marshal(map[string]interface{}{"company_id": 1, "graduate_name": "Test User"})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/graduate-employments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, newAdminJobController(nil, nil, gradRepo, audit).CreateGraduateEmployment, newCtx(req, rec), http.StatusOK)
	gradRepo.AssertExpectations(t)
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
			assertStatus(t, newAdminJobController(companyRepo, nil, nil, audit).CreateJobPosition, newCtx(req, rec), http.StatusOK)

			if created == nil {
				t.Fatal("CreateJobPosition が呼ばれていない")
			}
			if created.DataStatus != tt.wantStatus {
				t.Errorf("DataStatus = %q, want %q", created.DataStatus, tt.wantStatus)
			}
		})
	}
}
