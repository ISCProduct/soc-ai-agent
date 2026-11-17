package company_test

// CompanyRelationControllerのHTTPハンドラーテスト (Issue #432)
//
// 実行: cd Backend && go test ./internal/controllers/company/... -run CompanyRelation -v

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/companyfetch"
	companycontrollers "Backend/internal/controllers/company"
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"
	"Backend/internal/openai"
	companysvc "Backend/internal/services/company"
)

func newCompanyRelationController(repo *mocks.CompanyRelationQueryRepositoryMock) *companycontrollers.CompanyRelationController {
	return companycontrollers.NewCompanyRelationController(repo, nil)
}

// ---- GetCompanyRelations ----

func TestCompanyRelationController_GetCompanyRelations_Success(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	relations := []models.CompanyRelation{{RelationType: "capital_subsidiary"}}
	repo.On("GetByCompanyID", uint(1)).Return(relations, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/companies/1/relations", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newCompanyRelationController(repo).GetCompanyRelations, ctx, http.StatusOK)
	repo.AssertExpectations(t)
}

func TestCompanyRelationController_GetCompanyRelations_ServiceError(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	repo.On("GetByCompanyID", uint(1)).Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/companies/1/relations", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newCompanyRelationController(repo).GetCompanyRelations, ctx, http.StatusInternalServerError)
	repo.AssertExpectations(t)
}

// ---- GetCompanyMarketInfo ----

func TestCompanyRelationController_GetCompanyMarketInfo_Success(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	info := &models.CompanyMarketInfo{CompanyID: 1}
	repo.On("GetMarketInfoByCompanyID", uint(1)).Return(info, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/companies/1/market-info", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newCompanyRelationController(repo).GetCompanyMarketInfo, ctx, http.StatusOK)
	repo.AssertExpectations(t)
}

func TestCompanyRelationController_GetCompanyMarketInfo_NotFound(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	repo.On("GetMarketInfoByCompanyID", uint(1)).Return(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/companies/1/market-info", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newCompanyRelationController(repo).GetCompanyMarketInfo, ctx, http.StatusNotFound)
	repo.AssertExpectations(t)
}

func TestCompanyRelationController_GetCompanyMarketInfo_ServiceError(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	repo.On("GetMarketInfoByCompanyID", uint(1)).Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/companies/1/market-info", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newCompanyRelationController(repo).GetCompanyMarketInfo, ctx, http.StatusInternalServerError)
	repo.AssertExpectations(t)
}

// ---- GetAllCompanyRelations ----

func TestCompanyRelationController_GetAllCompanyRelations_Success(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	repo.On("GetAll").Return([]models.CompanyRelation{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/companies/relations/all", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newCompanyRelationController(repo).GetAllCompanyRelations, testsupport.NewCtx(req, rec), http.StatusOK)
	repo.AssertExpectations(t)
}

func TestCompanyRelationController_GetAllCompanyRelations_ServiceError(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	repo.On("GetAll").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/companies/relations/all", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newCompanyRelationController(repo).GetAllCompanyRelations, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
	repo.AssertExpectations(t)
}

// ---- GetAllMarketInfo ----

func TestCompanyRelationController_GetAllMarketInfo_Success(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	repo.On("GetAllMarketInfo").Return([]models.CompanyMarketInfo{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/companies/market-info/all", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newCompanyRelationController(repo).GetAllMarketInfo, testsupport.NewCtx(req, rec), http.StatusOK)
	repo.AssertExpectations(t)
}

// ---- GetCompanyByID ----

func TestCompanyRelationController_GetCompanyByID_NotFound(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	repo.On("GetCompanyByID", uint(1)).Return(nil, errors.New("not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/companies/1", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newCompanyRelationController(repo).GetCompanyByID, ctx, http.StatusNotFound)
	repo.AssertExpectations(t)
}

func TestCompanyRelationController_GetCompanyByID_Success(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	company := &models.Company{Name: "Test Corp"}
	repo.On("GetCompanyByID", uint(1)).Return(company, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/companies/1", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newCompanyRelationController(repo).GetCompanyByID, ctx, http.StatusOK)
	repo.AssertExpectations(t)
}

// ---- GetCompanyJobPositions ----

func TestCompanyRelationController_GetCompanyJobPositions_Success(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	positions := []models.CompanyJobPosition{{CompanyID: 1, Title: "Engineer"}}
	repo.On("GetJobPositionsByCompany", uint(1)).Return(positions, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/companies/1/job-positions", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newCompanyRelationController(repo).GetCompanyJobPositions, ctx, http.StatusOK)
	repo.AssertExpectations(t)
}

// ---- GetCompanies ----

func TestCompanyRelationController_GetCompanies_Success(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	companies := []models.Company{{Name: "Test Corp"}}
	repo.On("GetCompaniesFiltered", 10, 0, "", "", "").Return(companies, int64(1), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/companies", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newCompanyRelationController(repo).GetCompanies, testsupport.NewCtx(req, rec), http.StatusOK)
	repo.AssertExpectations(t)
}

func TestCompanyRelationController_GetCompanies_ServiceError(t *testing.T) {
	repo := &mocks.CompanyRelationQueryRepositoryMock{}
	repo.On("GetCompaniesFiltered", 10, 0, "", "", "").Return(nil, int64(0), errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/companies", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newCompanyRelationController(repo).GetCompanies, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
	repo.AssertExpectations(t)
}

type denyRelationSearchBudget struct{}

func (denyRelationSearchBudget) AllowSearch() error {
	return companyfetch.ErrSearchBudgetExceeded
}

func newValidatedRelationController(client *openai.Client, budget companyfetch.SearchBudget) *companycontrollers.CompanyRelationController {
	svc := companysvc.NewCompanyValidationService(nil, client)
	if budget != nil {
		svc.SetSearchBudget(budget)
	}
	ctrl := newCompanyRelationController(&mocks.CompanyRelationQueryRepositoryMock{})
	ctrl.SetCompanyValidator(svc)
	return ctrl
}

func TestCompanyRelationController_WebSearchCompanies_EmptyQuery(t *testing.T) {
	ctrl := newCompanyRelationController(&mocks.CompanyRelationQueryRepositoryMock{})
	req := httptest.NewRequest(http.MethodGet, "/api/companies/web-search", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, ctrl.WebSearchCompanies, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestCompanyRelationController_WebSearchCompanies_BudgetExceeded(t *testing.T) {
	ctrl := newValidatedRelationController(openai.NewWithBaseURL("http://127.0.0.1:1", "gpt-4o-mini"), denyRelationSearchBudget{})
	req := httptest.NewRequest(http.MethodGet, "/api/companies/web-search?q=架空電機", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, ctrl.WebSearchCompanies, testsupport.NewCtx(req, rec), http.StatusTooManyRequests)
	if strings.Contains(rec.Body.String(), "budget exceeded") {
		t.Fatalf("予算超過の内部文を返した: %s", rec.Body.String())
	}
}

func TestCompanyRelationController_WebSearchCompanies_UpstreamFailureHidesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "SECRET_UPSTREAM_BODY", http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	ctrl := newValidatedRelationController(openai.NewWithBaseURL(srv.URL, "gpt-4o-mini"), nil)
	req := httptest.NewRequest(http.MethodGet, "/api/companies/web-search?q=架空電機", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, ctrl.WebSearchCompanies, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
	if strings.Contains(rec.Body.String(), "SECRET_UPSTREAM_BODY") {
		t.Fatalf("上流の本文を返した: %s", rec.Body.String())
	}
}

func TestCompanyRelationController_WebSearchCompanies_NoCandidates(t *testing.T) {
	const body = `{"output_text":"{\"exists\":false,\"canonical_name\":\"\",\"evidence_urls\":[],\"confidence\":\"low\",\"description\":\"\"}"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	ctrl := newValidatedRelationController(openai.NewWithBaseURL(srv.URL, "gpt-4o-mini"), nil)
	req := httptest.NewRequest(http.MethodGet, "/api/companies/web-search?q=存在しない企業XYZ", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, ctrl.WebSearchCompanies, testsupport.NewCtx(req, rec), http.StatusOK)
	var payload struct {
		Results []any `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Results) != 0 {
		t.Fatalf("候補なしを期待: %s", rec.Body.String())
	}
}

func TestCompanyRelationController_ValidateCompany_BudgetExceededHidesSentinel(t *testing.T) {
	ctrl := newValidatedRelationController(openai.NewWithBaseURL("http://127.0.0.1:1", "gpt-4o-mini"), denyRelationSearchBudget{})
	req := httptest.NewRequest(http.MethodPost, "/api/companies/validate", strings.NewReader(`{"query":"架空電機"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, ctrl.ValidateCompany, testsupport.NewCtx(req, rec), http.StatusTooManyRequests)
	if strings.Contains(rec.Body.String(), "budget exceeded") || strings.Contains(rec.Body.String(), "127.0.0.1") {
		t.Fatalf("内部情報を返した: %s", rec.Body.String())
	}
}
