package admin_test

// AdminDashboardController・AdminCompanyGraphControllerのHTTPハンドラーテスト
//
// 実行: cd Backend && go test ./internal/controllers/admin/... -run "AdminDashboard|AdminCompanyGraph" -v

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"Backend/domain/entity"
	admincontrollers "Backend/internal/controllers/admin"
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/organization"
	"Backend/internal/services/school"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ========== AdminDashboardController ==========

func newAdminDashboardController(
	userRepo *mocks.UserRepositoryMock,
	sessRepo *mocks.DashboardSessionRepoMock,
	repRepo *mocks.DashboardReportRepoMock,
) *admincontrollers.AdminDashboardController {
	return admincontrollers.NewAdminDashboardController(userRepo, sessRepo, repRepo)
}

// ---- UserSessions ----

func TestAdminDashboardController_UserSessions_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users/abc/sessions", nil)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("abc")
	testsupport.AssertStatus(t, admincontrollers.NewAdminDashboardController(nil, nil, nil).UserSessions, c, http.StatusBadRequest)
}

func TestAdminDashboardController_UserSessions_SessionRepoError(t *testing.T) {
	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users/1/sessions", nil), 42)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("GetUserByID", uint(1)).Return(&entity.User{Email: "user@example.com"}, nil)
	sessRepo := &mocks.DashboardSessionRepoMock{}
	sessRepo.On("ListFinishedSessionIDsByUser", uint(1)).Return([]uint{}, errors.New("db error"))
	ctrl := newAdminDashboardController(userRepo, sessRepo, nil)
	ctrl.SetSchoolService(testsupport.NewUnrestrictedSchoolService(42))
	testsupport.AssertStatus(t, ctrl.UserSessions, c, http.StatusInternalServerError)
	sessRepo.AssertExpectations(t)
}

func TestAdminDashboardController_UserSessions_Success(t *testing.T) {
	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users/2/sessions", nil), 42)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("2")

	now := time.Now()
	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("GetUserByID", uint(2)).Return(&entity.User{Email: "user2@example.com"}, nil)
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

	ctrl := newAdminDashboardController(userRepo, sessRepo, repRepo)
	ctrl.SetSchoolService(testsupport.NewUnrestrictedSchoolService(42))
	testsupport.AssertStatus(t, ctrl.UserSessions, c, http.StatusOK)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body, "sessions")
	sessRepo.AssertExpectations(t)
	repRepo.AssertExpectations(t)
}

// #984: school scope制限のあるadminは、担当校外のユーザーのセッションを閲覧できない(403)。
func TestAdminDashboardController_UserSessions_SchoolAccessDenied(t *testing.T) {
	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users/3/sessions", nil), 42)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("3")

	otherSchoolID := uint(99)
	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("GetUserByID", uint(3)).Return(&entity.User{Email: "user3@example.com", SchoolID: &otherSchoolID}, nil)
	schoolRepo := &mocks.SchoolRepositoryMock{}
	schoolRepo.On("ListSchoolsForAdmin", uint(42)).Return([]models.School{{ID: 1}}, nil)

	ctrl := newAdminDashboardController(userRepo, nil, nil)
	ctrl.SetSchoolService(school.NewSchoolService(schoolRepo))
	testsupport.AssertStatus(t, ctrl.UserSessions, c, http.StatusForbidden)
	userRepo.AssertExpectations(t)
}

func TestAdminDashboardController_UserSessions_ReportRepoError(t *testing.T) {
	req := testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users/3/sessions", nil), 42)
	rec := httptest.NewRecorder()
	c := testsupport.NewCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("3")

	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("GetUserByID", uint(3)).Return(&entity.User{Email: "user3@example.com"}, nil)
	sessRepo := &mocks.DashboardSessionRepoMock{}
	repRepo := &mocks.DashboardReportRepoMock{}
	sessRepo.On("ListFinishedSessionIDsByUser", uint(3)).Return([]uint{20}, nil)
	repRepo.On("FindBySessionIDs", []uint{20}).Return([]models.InterviewReport{}, errors.New("db error"))
	ctrl := newAdminDashboardController(userRepo, sessRepo, repRepo)
	ctrl.SetSchoolService(testsupport.NewUnrestrictedSchoolService(42))
	testsupport.AssertStatus(t, ctrl.UserSessions, c, http.StatusInternalServerError)
}

// ---- ListUsers ----

func TestAdminDashboardController_ListUsers_UserRepoError(t *testing.T) {
	req := testsupport.WithSchoolFilter(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users", nil), nil)
	rec := httptest.NewRecorder()

	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("ListUsersPaged", 25, 0, "", mock.Anything).Return([]entity.User{}, int64(0), errors.New("db error"))
	testsupport.AssertStatus(t, newAdminDashboardController(userRepo, nil, nil).ListUsers, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
	userRepo.AssertExpectations(t)
}

func TestAdminDashboardController_ListUsers_Success(t *testing.T) {
	req := testsupport.WithSchoolFilter(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users?limit=10&page=1", nil), nil)
	rec := httptest.NewRecorder()

	users := []entity.User{{ID: 1, Name: "テストユーザー", Email: "test@example.com"}}
	userRepo := &mocks.UserRepositoryMock{}
	sessRepo := &mocks.DashboardSessionRepoMock{}
	repRepo := &mocks.DashboardReportRepoMock{}

	userRepo.On("ListUsersPaged", 10, 0, "", mock.Anything).Return(users, int64(1), nil)
	sessRepo.On("GetUserStatsBatch", []uint{1}).Return(map[uint]repositories.UserSessionStat{
		1: {UserID: 1, SessionCount: 3},
	}, nil)
	sessRepo.On("ListFinishedSessionIDsByUser", uint(1)).Return([]uint{}, nil)
	repRepo.On("FindBySessionIDs", []uint(nil)).Return([]models.InterviewReport{}, nil)

	testsupport.AssertStatus(t, newAdminDashboardController(userRepo, sessRepo, repRepo).ListUsers, testsupport.NewCtx(req, rec), http.StatusOK)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, float64(1), body["total"])
	userRepo.AssertExpectations(t)
}

func TestAdminDashboardController_ListUsers_SessionStatError(t *testing.T) {
	req := testsupport.WithSchoolFilter(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users", nil), nil)
	rec := httptest.NewRecorder()

	users := []entity.User{{ID: 1, Name: "ユーザー"}}
	userRepo := &mocks.UserRepositoryMock{}
	sessRepo := &mocks.DashboardSessionRepoMock{}

	userRepo.On("ListUsersPaged", 25, 0, "", mock.Anything).Return(users, int64(1), nil)
	sessRepo.On("GetUserStatsBatch", []uint{1}).Return(nil, errors.New("db error"))
	testsupport.AssertStatus(t, newAdminDashboardController(userRepo, sessRepo, nil).ListUsers, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
}

// ---- ExportCSV / currentAdminPlan fail-closed (#985 CodeRabbit指摘) ----
// 組織解決に失敗した場合、entitlement.CurrentPlan()(DEFAULT_PLAN未設定時はPlanPro)へ
// フォールバックすると特権機能(CSVエクスポート)が一時障害で素通りしてしまうため、
// fail-closed(PlanFree=export不可)であることを確認する。

func TestAdminDashboardController_ExportCSV_AdminIDMissing_FailsClosed(t *testing.T) {
	repo := &mocks.OrganizationRepositoryMock{}
	ctrl := newAdminDashboardController(nil, nil, nil)
	ctrl.SetOrganizationService(organization.NewOrganizationService(repo))

	req := testsupport.WithSchoolFilter(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/export/csv", nil), nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, ctrl.ExportCSV, testsupport.NewCtx(req, rec), http.StatusForbidden)
}

func TestAdminDashboardController_ExportCSV_OrgLookupError_FailsClosed(t *testing.T) {
	repo := &mocks.OrganizationRepositoryMock{}
	repo.On("FindMembershipByUserID", uint(42)).Return(nil, errors.New("db error"))
	ctrl := newAdminDashboardController(nil, nil, nil)
	ctrl.SetOrganizationService(organization.NewOrganizationService(repo))

	req := testsupport.WithSchoolFilter(testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/export/csv", nil), 42), nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, ctrl.ExportCSV, testsupport.NewCtx(req, rec), http.StatusForbidden)
}

func TestAdminDashboardController_ExportCSV_OrgUnassigned_UsesGlobalDefault(t *testing.T) {
	orgRepo := &mocks.OrganizationRepositoryMock{}
	orgRepo.On("FindMembershipByUserID", uint(42)).Return(nil, nil)
	orgRepo.On("GetUserOrganizationID", uint(42)).Return(uint(0), nil)
	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("ListUsersPaged", 10000, 0, "", mock.Anything).Return([]entity.User{}, int64(0), errors.New("unrelated db error"))
	ctrl := newAdminDashboardController(userRepo, nil, nil)
	ctrl.SetOrganizationService(organization.NewOrganizationService(orgRepo))

	req := testsupport.WithSchoolFilter(testsupport.WithAdminUserID(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/export/csv", nil), 42), nil)
	rec := httptest.NewRecorder()
	// orgID==0(プラットフォーム管理者)はCurrentPlan()(既定PlanPro)にフォールバックするため
	// export自体は許可される(fail-closedで403にはならない)。以降はuserRepoのエラーで500。
	testsupport.AssertStatus(t, ctrl.ExportCSV, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
}

// ---- ExportCSV ----

func TestAdminDashboardController_ExportCSV_UserRepoError(t *testing.T) {
	req := testsupport.WithSchoolFilter(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/export/csv", nil), nil)
	rec := httptest.NewRecorder()

	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("ListUsersPaged", 10000, 0, "", mock.Anything).Return([]entity.User{}, int64(0), errors.New("db error"))
	testsupport.AssertStatus(t, newAdminDashboardController(userRepo, nil, nil).ExportCSV, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
}

func TestAdminDashboardController_ExportCSV_Success(t *testing.T) {
	req := testsupport.WithSchoolFilter(httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/export/csv", nil), nil)
	rec := httptest.NewRecorder()

	users := []entity.User{
		{ID: 1, Name: "田中 太郎", Email: "tanaka@example.com", TargetLevel: "新卒"},
	}
	userRepo := &mocks.UserRepositoryMock{}
	sessRepo := &mocks.DashboardSessionRepoMock{}
	repRepo := &mocks.DashboardReportRepoMock{}

	userRepo.On("ListUsersPaged", 10000, 0, "", mock.Anything).Return(users, int64(1), nil)
	sessRepo.On("GetUserStatsBatch", []uint{1}).Return(map[uint]repositories.UserSessionStat{}, nil)
	sessRepo.On("ListFinishedSessionIDsByUser", uint(1)).Return([]uint{}, nil)
	repRepo.On("FindBySessionIDs", []uint(nil)).Return([]models.InterviewReport{}, nil)

	testsupport.AssertStatus(t, newAdminDashboardController(userRepo, sessRepo, repRepo).ExportCSV, testsupport.NewCtx(req, rec), http.StatusOK)

	assert.Equal(t, "text/csv; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Body.String(), "田中 太郎")
}

// ========== AdminCompanyGraphController ==========

func newAdminCompanyGraphController() *admincontrollers.AdminCompanyGraphController {
	return admincontrollers.NewAdminCompanyGraphController(nil, nil, nil, nil, nil)
}

// ---- TargetYear ----

func TestAdminCompanyGraphController_TargetYear_NoParam(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/company-graph/target-year", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminCompanyGraphController().TargetYear, testsupport.NewCtx(req, rec), http.StatusOK)

	var body map[string]int
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Greater(t, body["target_year"], 2000)
}

func TestAdminCompanyGraphController_TargetYear_WithYear(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/company-graph/target-year?year=2023", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminCompanyGraphController().TargetYear, testsupport.NewCtx(req, rec), http.StatusOK)

	var body map[string]int
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, 2023, body["target_year"])
}

// ---- Crawl (nil pipeline) ----

func TestAdminCompanyGraphController_Crawl_NilPipeline(t *testing.T) {
	// COMPANY_GRAPH_URL が未設定かつ pipeline が nil → 500
	req := httptest.NewRequest(http.MethodPost, "/api/admin/company-graph/crawl", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminCompanyGraphController().Crawl, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
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
	testsupport.AssertStatus(t, newAdminCompanyGraphController().Crawl, testsupport.NewCtx(req, rec), http.StatusBadGateway)
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
	testsupport.AssertStatus(t, newAdminCompanyGraphController().Crawl, testsupport.NewCtx(req, rec), http.StatusOK)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["ok"])
}
