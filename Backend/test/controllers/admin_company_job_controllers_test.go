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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ===== AdminCompanyController =====

func newAdminCompanyController(repo *mocks.CompanyRepositoryMock, audit *mocks.AuditLogServiceMock) *controllers.AdminCompanyController {
	return controllers.NewAdminCompanyController(repo, audit, nil)
}

func TestAdminCompanyController_ListOrCreate_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/companies", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminCompanyController(nil, nil, nil).ListOrCreate(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminCompanyController_List_ServiceError(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindAllActive", 50, 0).Return(nil, errors.New("db error"))
	repo.On("CountActive").Return(int64(0), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies", nil)
	w := httptest.NewRecorder()
	newAdminCompanyController(repo, nil).ListOrCreate(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAdminCompanyController_List_Success(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindAllActive", 50, 0).Return([]models.Company{{Name: "Test Corp"}}, nil)
	repo.On("CountActive").Return(int64(1), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies", nil)
	w := httptest.NewRecorder()
	newAdminCompanyController(repo, nil).ListOrCreate(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Create_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/companies", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	controllers.NewAdminCompanyController(nil, nil, nil).ListOrCreate(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminCompanyController_Create_MissingName(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"prefecture": "東京都"})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/companies", bytes.NewReader(body))
	w := httptest.NewRecorder()
	controllers.NewAdminCompanyController(nil, nil, nil).ListOrCreate(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminCompanyController_Create_Success(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	repo.On("Create", mock.Anything).Return(nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	body, _ := json.Marshal(map[string]string{"name": "Test Corp"})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/companies", bytes.NewReader(body))
	w := httptest.NewRecorder()
	newAdminCompanyController(repo, audit).ListOrCreate(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Detail_EmptyID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies/", nil)
	req.URL.Path = "/api/admin/companies/"
	w := httptest.NewRecorder()
	controllers.NewAdminCompanyController(nil, nil, nil).Detail(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminCompanyController_Detail_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies/abc", nil)
	req.URL.Path = "/api/admin/companies/abc"
	w := httptest.NewRecorder()
	controllers.NewAdminCompanyController(nil, nil, nil).Detail(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminCompanyController_Detail_Get_NotFound(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(nil, errors.New("not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies/1", nil)
	req.URL.Path = "/api/admin/companies/1"
	w := httptest.NewRecorder()
	newAdminCompanyController(repo, nil).Detail(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Detail_Get_Success(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	company := &models.Company{Name: "Test Corp"}
	repo.On("FindByID", uint(1)).Return(company, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies/1", nil)
	req.URL.Path = "/api/admin/companies/1"
	w := httptest.NewRecorder()
	newAdminCompanyController(repo, nil).Detail(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Detail_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/companies/1", nil)
	req.URL.Path = "/api/admin/companies/1"
	w := httptest.NewRecorder()
	controllers.NewAdminCompanyController(nil, nil, nil).Detail(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminCompanyController_SearchGBizRoute_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/companies/search-gbiz", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminCompanyController(nil, nil, nil).SearchGBizRoute(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

// ===== AdminJobController =====

func newAdminJobController(companyRepo *mocks.CompanyRepositoryMock, jobCatRepo *mocks.JobCategoryRepositoryMock, gradRepo *mocks.GraduateEmploymentRepositoryMock, audit *mocks.AuditLogServiceMock) *controllers.AdminJobController {
	return controllers.NewAdminJobController(companyRepo, jobCatRepo, gradRepo, audit)
}

func TestAdminJobController_JobCategories_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/job-categories", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminJobController(nil, nil, nil, nil).JobCategories(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminJobController_JobCategories_ServiceError(t *testing.T) {
	jobCatRepo := &mocks.JobCategoryRepositoryMock{}
	jobCatRepo.On("FindAll").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/job-categories", nil)
	w := httptest.NewRecorder()
	newAdminJobController(nil, jobCatRepo, nil, nil).JobCategories(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	jobCatRepo.AssertExpectations(t)
}

func TestAdminJobController_JobCategories_Success(t *testing.T) {
	jobCatRepo := &mocks.JobCategoryRepositoryMock{}
	categories := []models.JobCategory{{Name: "Engineer"}}
	jobCatRepo.On("FindAll").Return(categories, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/job-categories", nil)
	w := httptest.NewRecorder()
	newAdminJobController(nil, jobCatRepo, nil, nil).JobCategories(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	jobCatRepo.AssertExpectations(t)
}

func TestAdminJobController_JobPositions_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/job-positions?company_id=1", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminJobController(nil, nil, nil, nil).JobPositions(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminJobController_JobPositions_List_Success(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	positions := []models.CompanyJobPosition{{Title: "Software Engineer"}}
	companyRepo.On("ListJobPositions", (*uint)(nil), 50).Return(positions, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/job-positions", nil)
	w := httptest.NewRecorder()
	newAdminJobController(companyRepo, nil, nil, nil).JobPositions(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	companyRepo.AssertExpectations(t)
}

func TestAdminJobController_GraduateEmployments_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/graduate-employments", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminJobController(nil, nil, nil, nil).GraduateEmployments(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminJobController_GraduateEmployments_List_Success(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	entries := []models.GraduateEmployment{{GraduateName: "Test User"}}
	gradRepo.On("List", (*uint)(nil), 50).Return(entries, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/graduate-employments", nil)
	w := httptest.NewRecorder()
	newAdminJobController(nil, nil, gradRepo, nil).GraduateEmployments(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	gradRepo.AssertExpectations(t)
}

func TestAdminJobController_GraduateEmploymentDetail_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/graduate-employments/1", nil)
	req.URL.Path = "/api/admin/graduate-employments/1"
	w := httptest.NewRecorder()
	controllers.NewAdminJobController(nil, nil, nil, nil).GraduateEmploymentDetail(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminJobController_GraduateEmploymentDetail_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/graduate-employments/abc", nil)
	req.URL.Path = "/api/admin/graduate-employments/abc"
	w := httptest.NewRecorder()
	controllers.NewAdminJobController(nil, nil, nil, nil).GraduateEmploymentDetail(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminJobController_GraduateEmploymentDetail_NotFound(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	gradRepo.On("FindByID", uint(1)).Return(nil, errors.New("not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/graduate-employments/1", nil)
	req.URL.Path = "/api/admin/graduate-employments/1"
	w := httptest.NewRecorder()
	newAdminJobController(nil, nil, gradRepo, nil).GraduateEmploymentDetail(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	gradRepo.AssertExpectations(t)
}

func TestAdminJobController_GraduateEmploymentDetail_Get_Success(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	entry := &models.GraduateEmployment{GraduateName: "Test User"}
	gradRepo.On("FindByID", uint(1)).Return(entry, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/graduate-employments/1", nil)
	req.URL.Path = "/api/admin/graduate-employments/1"
	w := httptest.NewRecorder()
	newAdminJobController(nil, nil, gradRepo, nil).GraduateEmploymentDetail(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	gradRepo.AssertExpectations(t)
}

func TestAdminJobController_GraduateEmploymentDetail_Put_Success(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	entry := &models.GraduateEmployment{GraduateName: "Test User", CompanyID: 1}
	gradRepo.On("FindByID", uint(1)).Return(entry, nil)
	gradRepo.On("Update", mock.Anything).Return(nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	body, _ := json.Marshal(map[string]interface{}{"company_id": 1, "graduate_name": "Updated User"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/graduate-employments/1", bytes.NewReader(body))
	req.URL.Path = "/api/admin/graduate-employments/1"
	w := httptest.NewRecorder()
	newAdminJobController(nil, nil, gradRepo, audit).GraduateEmploymentDetail(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	gradRepo.AssertExpectations(t)
}

func TestAdminJobController_JobPositionAction_InvalidPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/job-positions/1", nil)
	req.URL.Path = "/api/admin/job-positions/1"
	w := httptest.NewRecorder()
	controllers.NewAdminJobController(nil, nil, nil, nil).JobPositionAction(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminJobController_JobPositionAction_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/job-positions/abc/publish", nil)
	req.URL.Path = "/api/admin/job-positions/abc/publish"
	w := httptest.NewRecorder()
	controllers.NewAdminJobController(nil, nil, nil, nil).JobPositionAction(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminJobController_JobPositionAction_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/job-positions/1/publish", nil)
	req.URL.Path = "/api/admin/job-positions/1/publish"
	w := httptest.NewRecorder()
	controllers.NewAdminJobController(nil, nil, nil, nil).JobPositionAction(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminJobController_JobPositionAction_NotFound(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	companyRepo.On("FindJobPositionByID", uint(1)).Return(nil, errors.New("not found"))

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/job-positions/1/publish", nil)
	req.URL.Path = "/api/admin/job-positions/1/publish"
	w := httptest.NewRecorder()
	newAdminJobController(companyRepo, nil, nil, nil).JobPositionAction(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	companyRepo.AssertExpectations(t)
}

func TestAdminJobController_JobPositionAction_UnknownAction(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	position := &models.CompanyJobPosition{Title: "Engineer"}
	companyRepo.On("FindJobPositionByID", uint(1)).Return(position, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/job-positions/1/unknown", nil)
	req.URL.Path = "/api/admin/job-positions/1/unknown"
	w := httptest.NewRecorder()
	newAdminJobController(companyRepo, nil, nil, nil).JobPositionAction(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	companyRepo.AssertExpectations(t)
}

func TestAdminJobController_JobPositionAction_Publish_Success(t *testing.T) {
	companyRepo := &mocks.CompanyRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	position := &models.CompanyJobPosition{Title: "Engineer"}
	companyRepo.On("FindJobPositionByID", uint(1)).Return(position, nil)
	companyRepo.On("UpdateJobPosition", mock.Anything).Return(nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/job-positions/1/publish", nil)
	req.URL.Path = "/api/admin/job-positions/1/publish"
	w := httptest.NewRecorder()
	newAdminJobController(companyRepo, nil, nil, audit).JobPositionAction(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	companyRepo.AssertExpectations(t)
}

func TestAdminJobController_JobPositionAction_Create_InvalidBody(t *testing.T) {
	// POSTのcreateはJobPositionActionではなく別のルートだがJobPositionsはGETのみ
	// JobPositions POST → MethodNotAllowed (実装上GET専用)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/job-positions", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	controllers.NewAdminJobController(nil, nil, nil, nil).JobPositions(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminJobController_GraduateEmployments_Create_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/graduate-employments", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	controllers.NewAdminJobController(nil, nil, nil, nil).GraduateEmployments(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminJobController_GraduateEmployments_Create_MissingCompanyID(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"graduate_name": "Test"})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/graduate-employments", bytes.NewReader(body))
	w := httptest.NewRecorder()
	controllers.NewAdminJobController(nil, nil, nil, nil).GraduateEmployments(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminJobController_GraduateEmployments_Create_Success(t *testing.T) {
	gradRepo := &mocks.GraduateEmploymentRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	gradRepo.On("Create", mock.Anything).Return(nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	body, _ := json.Marshal(map[string]interface{}{"company_id": 1, "graduate_name": "Test User"})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/graduate-employments", bytes.NewReader(body))
	w := httptest.NewRecorder()
	newAdminJobController(nil, nil, gradRepo, audit).GraduateEmployments(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	gradRepo.AssertExpectations(t)
}
