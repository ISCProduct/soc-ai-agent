package controllers_test

// CompanyRelationControllerのHTTPハンドラーテスト (Issue #432)
//
// 実行: cd Backend && go test ./test/controllers/... -run CompanyRelation -v

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	"Backend/internal/models"
	"Backend/test/controllers/mocks"
)

func newCompanyRelationController(repo *mocks.CompanyRelationQueryRepositoryMock) *controllers.CompanyRelationController {
	return controllers.NewCompanyRelationController(repo, nil)
}

// ---- GetCompanyRelations ----

func TestCompanyRelationController_GetCompanyRelations_Success(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	relations := []models.CompanyRelation{{RelationType: "capital_subsidiary"}}
	repo.On("GetByCompanyID", uint(1)).Return(relations, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/companies/1/relations", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newCompanyRelationController(repo).GetCompanyRelations, ctx, http.StatusOK)
	repo.AssertExpectations(t)
}

func TestCompanyRelationController_GetCompanyRelations_ServiceError(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	repo.On("GetByCompanyID", uint(1)).Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/companies/1/relations", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newCompanyRelationController(repo).GetCompanyRelations, ctx, http.StatusInternalServerError)
	repo.AssertExpectations(t)
}

// ---- GetCompanyMarketInfo ----

func TestCompanyRelationController_GetCompanyMarketInfo_Success(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	info := &models.CompanyMarketInfo{CompanyID: 1}
	repo.On("GetMarketInfoByCompanyID", uint(1)).Return(info, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/companies/1/market-info", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newCompanyRelationController(repo).GetCompanyMarketInfo, ctx, http.StatusOK)
	repo.AssertExpectations(t)
}

func TestCompanyRelationController_GetCompanyMarketInfo_NotFound(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	repo.On("GetMarketInfoByCompanyID", uint(1)).Return(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/companies/1/market-info", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newCompanyRelationController(repo).GetCompanyMarketInfo, ctx, http.StatusNotFound)
	repo.AssertExpectations(t)
}

func TestCompanyRelationController_GetCompanyMarketInfo_ServiceError(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	repo.On("GetMarketInfoByCompanyID", uint(1)).Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/companies/1/market-info", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newCompanyRelationController(repo).GetCompanyMarketInfo, ctx, http.StatusInternalServerError)
	repo.AssertExpectations(t)
}

// ---- GetAllCompanyRelations ----

func TestCompanyRelationController_GetAllCompanyRelations_Success(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	repo.On("GetAll").Return([]models.CompanyRelation{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/companies/relations/all", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newCompanyRelationController(repo).GetAllCompanyRelations, newCtx(req, rec), http.StatusOK)
	repo.AssertExpectations(t)
}

func TestCompanyRelationController_GetAllCompanyRelations_ServiceError(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	repo.On("GetAll").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/companies/relations/all", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newCompanyRelationController(repo).GetAllCompanyRelations, newCtx(req, rec), http.StatusInternalServerError)
	repo.AssertExpectations(t)
}

// ---- GetAllMarketInfo ----

func TestCompanyRelationController_GetAllMarketInfo_Success(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	repo.On("GetAllMarketInfo").Return([]models.CompanyMarketInfo{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/companies/market-info/all", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newCompanyRelationController(repo).GetAllMarketInfo, newCtx(req, rec), http.StatusOK)
	repo.AssertExpectations(t)
}

// ---- GetCompanyByID ----

func TestCompanyRelationController_GetCompanyByID_NotFound(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	repo.On("GetCompanyByID", uint(1)).Return(nil, errors.New("not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/companies/1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newCompanyRelationController(repo).GetCompanyByID, ctx, http.StatusNotFound)
	repo.AssertExpectations(t)
}

func TestCompanyRelationController_GetCompanyByID_Success(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	company := &models.Company{Name: "Test Corp"}
	repo.On("GetCompanyByID", uint(1)).Return(company, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/companies/1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newCompanyRelationController(repo).GetCompanyByID, ctx, http.StatusOK)
	repo.AssertExpectations(t)
}

// ---- GetCompanyJobPositions ----

func TestCompanyRelationController_GetCompanyJobPositions_Success(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	positions := []models.CompanyJobPosition{{CompanyID: 1, Title: "Engineer"}}
	repo.On("GetJobPositionsByCompany", uint(1)).Return(positions, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/companies/1/job-positions", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newCompanyRelationController(repo).GetCompanyJobPositions, ctx, http.StatusOK)
	repo.AssertExpectations(t)
}

// ---- GetCompanies ----

func TestCompanyRelationController_GetCompanies_Success(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	companies := []models.Company{{Name: "Test Corp"}}
	repo.On("GetCompaniesFiltered", 10, 0, "", "", "").Return(companies, int64(1), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/companies", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newCompanyRelationController(repo).GetCompanies, newCtx(req, rec), http.StatusOK)
	repo.AssertExpectations(t)
}

func TestCompanyRelationController_GetCompanies_ServiceError(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	repo.On("GetCompaniesFiltered", 10, 0, "", "", "").Return(nil, int64(0), errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/companies", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newCompanyRelationController(repo).GetCompanies, newCtx(req, rec), http.StatusInternalServerError)
	repo.AssertExpectations(t)
}
