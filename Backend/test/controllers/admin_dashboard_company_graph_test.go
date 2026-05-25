package controllers_test

// AdminDashboardController・AdminCompanyGraphControllerのHTTPハンドラーテスト
//
// 実行: cd Backend && go test ./test/controllers/... -run "AdminDashboard|AdminCompanyGraph" -v

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"Backend/domain/entity"
	"Backend/internal/controllers"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
)

// ========== AdminDashboardController ==========

func newAdminDashboardController(
	userRepo *mocks.UserRepositoryMock,
	sessRepo *mocks.DashboardSessionRepoMock,
	repRepo *mocks.DashboardReportRepoMock,
) *controllers.AdminDashboardController {
	return controllers.NewAdminDashboardController(userRepo, sessRepo, repRepo)
}

// ---- UserSessions ----

func TestAdminDashboardController_UserSessions_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users/abc/sessions", nil)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("abc")
	assertStatus(t, controllers.NewAdminDashboardController(nil, nil, nil).UserSessions, c, http.StatusBadRequest)
}

func TestAdminDashboardController_UserSessions_SessionRepoError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users/1/sessions", nil)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	sessRepo := &mocks.DashboardSessionRepoMock{}
	sessRepo.On("ListFinishedSessionIDsByUser", uint(1)).Return([]uint{}, errors.New("db error"))
	assertStatus(t, newAdminDashboardController(nil, sessRepo, nil).UserSessions, c, http.StatusInternalServerError)
	sessRepo.AssertExpectations(t)
}

func TestAdminDashboardController_UserSessions_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users/2/sessions", nil)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("2")

	now := time.Now()
	sessRepo := &mocks.DashboardSessionRepoMock{}
	repRepo := &mocks.DashboardReportRepoMock{}
	sessRepo.On("ListFinishedSessionIDsByUser", uint(2)).Return([]uint{10, 11}, nil)
	repRepo.On("FindBySessionIDs", []uint{10, 11}).Return([]models.InterviewReport{
		{SessionID: 10, ScoresJSON: `{"logic":4,"specificity":3}`},
	}, nil)
	sessRepo.On("ListFinishedByUser", uint(2), 0).Return([]models.InterviewSession{
		{ID: 10, EndedAt: &now},
		{ID: 11, EndedAt: &now},
	}, nil)

	assertStatus(t, newAdminDashboardController(nil, sessRepo, repRepo).UserSessions, c, http.StatusOK)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body, "sessions")
	sessRepo.AssertExpectations(t)
	repRepo.AssertExpectations(t)
}

func TestAdminDashboardController_UserSessions_ReportRepoError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users/3/sessions", nil)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("3")

	sessRepo := &mocks.DashboardSessionRepoMock{}
	repRepo := &mocks.DashboardReportRepoMock{}
	sessRepo.On("ListFinishedSessionIDsByUser", uint(3)).Return([]uint{20}, nil)
	repRepo.On("FindBySessionIDs", []uint{20}).Return([]models.InterviewReport{}, errors.New("db error"))
	assertStatus(t, newAdminDashboardController(nil, sessRepo, repRepo).UserSessions, c, http.StatusInternalServerError)
}

// ---- ListUsers ----

func TestAdminDashboardController_ListUsers_UserRepoError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users", nil)
	rec := httptest.NewRecorder()

	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("ListUsersPaged", 25, 0, "").Return([]entity.User{}, int64(0), errors.New("db error"))
	assertStatus(t, newAdminDashboardController(userRepo, nil, nil).ListUsers, newCtx(req, rec), http.StatusInternalServerError)
	userRepo.AssertExpectations(t)
}

func TestAdminDashboardController_ListUsers_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users?limit=10&page=1", nil)
	rec := httptest.NewRecorder()

	users := []entity.User{{ID: 1, Name: "テストユーザー", Email: "test@example.com"}}
	userRepo := &mocks.UserRepositoryMock{}
	sessRepo := &mocks.DashboardSessionRepoMock{}
	repRepo := &mocks.DashboardReportRepoMock{}

	userRepo.On("ListUsersPaged", 10, 0, "").Return(users, int64(1), nil)
	sessRepo.On("GetUserStatsBatch", []uint{1}).Return(map[uint]repositories.UserSessionStat{
		1: {UserID: 1, SessionCount: 3},
	}, nil)
	sessRepo.On("ListFinishedSessionIDsByUser", uint(1)).Return([]uint{}, nil)
	repRepo.On("FindBySessionIDs", []uint(nil)).Return([]models.InterviewReport{}, nil)

	assertStatus(t, newAdminDashboardController(userRepo, sessRepo, repRepo).ListUsers, newCtx(req, rec), http.StatusOK)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, float64(1), body["total"])
	userRepo.AssertExpectations(t)
}

func TestAdminDashboardController_ListUsers_SessionStatError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users", nil)
	rec := httptest.NewRecorder()

	users := []entity.User{{ID: 1, Name: "ユーザー"}}
	userRepo := &mocks.UserRepositoryMock{}
	sessRepo := &mocks.DashboardSessionRepoMock{}

	userRepo.On("ListUsersPaged", 25, 0, "").Return(users, int64(1), nil)
	sessRepo.On("GetUserStatsBatch", []uint{1}).Return(nil, errors.New("db error"))
	assertStatus(t, newAdminDashboardController(userRepo, sessRepo, nil).ListUsers, newCtx(req, rec), http.StatusInternalServerError)
}

// ---- ExportCSV ----

func TestAdminDashboardController_ExportCSV_UserRepoError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/export/csv", nil)
	rec := httptest.NewRecorder()

	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("ListUsersPaged", 10000, 0, "").Return([]entity.User{}, int64(0), errors.New("db error"))
	assertStatus(t, newAdminDashboardController(userRepo, nil, nil).ExportCSV, newCtx(req, rec), http.StatusInternalServerError)
}

func TestAdminDashboardController_ExportCSV_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/export/csv", nil)
	rec := httptest.NewRecorder()

	users := []entity.User{
		{ID: 1, Name: "田中 太郎", Email: "tanaka@example.com", TargetLevel: "新卒"},
	}
	userRepo := &mocks.UserRepositoryMock{}
	sessRepo := &mocks.DashboardSessionRepoMock{}
	repRepo := &mocks.DashboardReportRepoMock{}

	userRepo.On("ListUsersPaged", 10000, 0, "").Return(users, int64(1), nil)
	sessRepo.On("GetUserStatsBatch", []uint{1}).Return(map[uint]repositories.UserSessionStat{}, nil)
	sessRepo.On("ListFinishedSessionIDsByUser", uint(1)).Return([]uint{}, nil)
	repRepo.On("FindBySessionIDs", []uint(nil)).Return([]models.InterviewReport{}, nil)

	assertStatus(t, newAdminDashboardController(userRepo, sessRepo, repRepo).ExportCSV, newCtx(req, rec), http.StatusOK)

	assert.Equal(t, "text/csv; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Body.String(), "田中 太郎")
}

// ========== AdminCompanyGraphController ==========

func newAdminCompanyGraphController() *controllers.AdminCompanyGraphController {
	return controllers.NewAdminCompanyGraphController(nil, nil, nil, nil)
}

// ---- TargetYear ----

func TestAdminCompanyGraphController_TargetYear_NoParam(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/company-graph/target-year", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newAdminCompanyGraphController().TargetYear, newCtx(req, rec), http.StatusOK)

	var body map[string]int
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Greater(t, body["target_year"], 2000)
}

func TestAdminCompanyGraphController_TargetYear_WithYear(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/company-graph/target-year?year=2023", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newAdminCompanyGraphController().TargetYear, newCtx(req, rec), http.StatusOK)

	var body map[string]int
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, 2023, body["target_year"])
}

// ---- Crawl (nil pipeline) ----

func TestAdminCompanyGraphController_Crawl_NilPipeline(t *testing.T) {
	// COMPANY_GRAPH_URL が未設定かつ pipeline が nil → 500
	req := httptest.NewRequest(http.MethodPost, "/api/admin/company-graph/crawl", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newAdminCompanyGraphController().Crawl, newCtx(req, rec), http.StatusInternalServerError)
}

func TestAdminCompanyGraphController_Crawl_ExternalServiceError(t *testing.T) {
	// COMPANY_GRAPH_URL をモックサーバーで上書きしてエラーレスポンスをシミュレート
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":false,"error":"crawl failed","logs":"","target_year":2024}`))
	}))
	defer server.Close()

	t.Setenv("COMPANY_GRAPH_URL", server.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/company-graph/crawl", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newAdminCompanyGraphController().Crawl, newCtx(req, rec), http.StatusBadGateway)
}

func TestAdminCompanyGraphController_Crawl_ExternalServiceSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true,"logs":"done","target_year":2024,"nodes":{}}`))
	}))
	defer server.Close()

	t.Setenv("COMPANY_GRAPH_URL", server.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/company-graph/crawl", nil)
	rec := httptest.NewRecorder()

	// companyRepo/relationRepo が nil → upsertNodes/syncRelationsFromNodes は 0 を返す
	assertStatus(t, newAdminCompanyGraphController().Crawl, newCtx(req, rec), http.StatusOK)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["ok"])
}
