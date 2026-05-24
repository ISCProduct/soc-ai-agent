package controllers_test

// Admin系軽量コントローラーのHTTPハンドラーテスト (Issue #429)
//
// 対象: AdminAuditController, AdminScraperSessionController,
//       AdminScoreValidationController, AdminProfileRecalculationController,
//       AdminUserController
//
// 実行: cd Backend && go test ./test/controllers/... -run "AdminAudit|AdminScraper|AdminScore|AdminProfile|AdminUser" -v

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/controllers"
	"Backend/internal/models"
	"Backend/internal/services"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/mock"
)

// ===== AdminAuditController =====

func TestAdminAuditController_List_ServiceError(t *testing.T) {
	svc := &mocks.AuditLogServiceMock{}
	svc.On("List", 50).Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/audit-logs", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminAuditController(svc).List, newCtx(req, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

func TestAdminAuditController_List_Success(t *testing.T) {
	svc := &mocks.AuditLogServiceMock{}
	logs := []models.AuditLog{{ActorEmail: "admin@example.com", Action: "user.update"}}
	svc.On("List", 50).Return(logs, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/audit-logs", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminAuditController(svc).List, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestAdminAuditController_List_CustomLimit(t *testing.T) {
	svc := &mocks.AuditLogServiceMock{}
	svc.On("List", 10).Return([]models.AuditLog{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/audit-logs?limit=10", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminAuditController(svc).List, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

// ===== AdminScraperSessionController =====

func TestAdminScraperSessionController_Sessions_List_ServiceError(t *testing.T) {
	svc := &mocks.ScraperSessionServiceMock{}
	svc.On("List").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/scraper-sessions", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminScraperSessionController(svc).List, newCtx(req, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

func TestAdminScraperSessionController_Sessions_List_Success(t *testing.T) {
	svc := &mocks.ScraperSessionServiceMock{}
	svc.On("List").Return([]models.ScraperSession{{SiteKey: "mynavi"}}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/scraper-sessions", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminScraperSessionController(svc).List, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestAdminScraperSessionController_Sessions_Upsert_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/scraper-sessions", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminScraperSessionController(nil).Upsert, newCtx(req, rec), http.StatusBadRequest)
}

func TestAdminScraperSessionController_Sessions_Upsert_InvalidExpiresAt(t *testing.T) {
	expiresAt := "not-a-date"
	body, _ := json.Marshal(map[string]interface{}{
		"site_key":   "mynavi",
		"cookies":    "session=abc",
		"expires_at": expiresAt,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/scraper-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminScraperSessionController(nil).Upsert, newCtx(req, rec), http.StatusBadRequest)
}

func TestAdminScraperSessionController_Sessions_Upsert_Success(t *testing.T) {
	svc := &mocks.ScraperSessionServiceMock{}
	session := &models.ScraperSession{SiteKey: "mynavi"}
	svc.On("Upsert", mock.Anything).Return(session, nil)

	body, _ := json.Marshal(map[string]string{
		"site_key": "mynavi",
		"cookies":  "session=abc",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/scraper-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminScraperSessionController(svc).Upsert, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestAdminScraperSessionController_SessionDetail_MissingSiteKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/scraper-sessions/", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("site_key")
	ctx.SetParamValues("")
	assertStatus(t, controllers.NewAdminScraperSessionController(nil).Delete, ctx, http.StatusBadRequest)
}

func TestAdminScraperSessionController_SessionDetail_ServiceError(t *testing.T) {
	svc := &mocks.ScraperSessionServiceMock{}
	svc.On("Delete", "mynavi").Return(errors.New("not found"))

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/scraper-sessions/mynavi", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("site_key")
	ctx.SetParamValues("mynavi")
	assertStatus(t, controllers.NewAdminScraperSessionController(svc).Delete, ctx, http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

func TestAdminScraperSessionController_SessionDetail_Success(t *testing.T) {
	svc := &mocks.ScraperSessionServiceMock{}
	svc.On("Delete", "mynavi").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/scraper-sessions/mynavi", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("site_key")
	ctx.SetParamValues("mynavi")
	assertStatus(t, controllers.NewAdminScraperSessionController(svc).Delete, ctx, http.StatusNoContent)
	svc.AssertExpectations(t)
}

// ===== AdminScoreValidationController =====

func TestAdminScoreValidationController_GetCorrelation_Success(t *testing.T) {
	svc := &mocks.ScoreValidationServiceMock{}
	svc.On("GetCorrelationReport").Return(&services.CorrelationReport{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/score-validation/correlation", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminScoreValidationController(svc).GetCorrelation, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestAdminScoreValidationController_GetCorrelation_Error(t *testing.T) {
	svc := &mocks.ScoreValidationServiceMock{}
	svc.On("GetCorrelationReport").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/score-validation/correlation", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminScoreValidationController(svc).GetCorrelation, newCtx(req, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

func TestAdminScoreValidationController_GetCalibration_Success(t *testing.T) {
	svc := &mocks.ScoreValidationServiceMock{}
	svc.On("GetCurrentCalibration").Return([]models.ScoreCalibrationWeight{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/score-validation/calibration", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminScoreValidationController(svc).GetCalibration, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestAdminScoreValidationController_RunCalibration_Success(t *testing.T) {
	svc := &mocks.ScoreValidationServiceMock{}
	svc.On("RunCalibration").Return(&services.CalibrationResult{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/score-validation/calibration/run", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminScoreValidationController(svc).RunCalibration, newCtx(req, rec), http.StatusCreated)
	svc.AssertExpectations(t)
}

func TestAdminScoreValidationController_ListVariants_Success(t *testing.T) {
	svc := &mocks.ScoreValidationServiceMock{}
	svc.On("ListExperiments").Return([]string{"exp-a", "exp-b"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/score-validation/variants", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminScoreValidationController(svc).ListVariants, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestAdminScoreValidationController_CreateVariant_Success(t *testing.T) {
	svc := &mocks.ScoreValidationServiceMock{}
	variant := &models.QuestionVariant{ExperimentName: "exp-a", VariantName: "control"}
	svc.On("CreateVariant", "exp-a", "control", "", 0.5).Return(variant, nil)

	body, _ := json.Marshal(map[string]string{
		"experiment_name": "exp-a",
		"variant_name":    "control",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/score-validation/variants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminScoreValidationController(svc).CreateVariant, newCtx(req, rec), http.StatusCreated)
	svc.AssertExpectations(t)
}

func TestAdminScoreValidationController_GetVariantResults_MissingExperiment(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/score-validation/variants/results", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminScoreValidationController(nil).GetVariantResults, newCtx(req, rec), http.StatusBadRequest)
}

// ===== AdminProfileRecalculationController =====

func TestAdminProfileRecalculationController_RecalculateOne_InvalidCompanyID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/profile-recalculation/abc", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("company_id")
	ctx.SetParamValues("abc")
	assertStatus(t, controllers.NewAdminProfileRecalculationController(nil).RecalculateOne, ctx, http.StatusBadRequest)
}

func TestAdminProfileRecalculationController_RecalculateAll_Success(t *testing.T) {
	svc := &mocks.ProfileRecalculationServiceMock{}
	svc.On("RecalculateAll", 0).Return([]*services.RecalculationResult{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/profile-recalculation", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminProfileRecalculationController(svc).RecalculateAll, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestAdminProfileRecalculationController_RecalculateOne_Success(t *testing.T) {
	svc := &mocks.ProfileRecalculationServiceMock{}
	svc.On("RecalculateCompany", uint(1), 0).Return(&services.RecalculationResult{CompanyID: 1}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/profile-recalculation/1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("company_id")
	ctx.SetParamValues("1")
	assertStatus(t, controllers.NewAdminProfileRecalculationController(svc).RecalculateOne, ctx, http.StatusOK)
	svc.AssertExpectations(t)
}

func TestAdminProfileRecalculationController_Rollback_Success(t *testing.T) {
	svc := &mocks.ProfileRecalculationServiceMock{}
	svc.On("Rollback", uint(1)).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/profile-recalculation/1/rollback", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("company_id")
	ctx.SetParamValues("1")
	assertStatus(t, controllers.NewAdminProfileRecalculationController(svc).Rollback, ctx, http.StatusOK)
	svc.AssertExpectations(t)
}

func TestAdminProfileRecalculationController_GetHistory_Success(t *testing.T) {
	svc := &mocks.ProfileRecalculationServiceMock{}
	svc.On("GetHistory", uint(1)).Return([]*models.CompanyProfileUpdateHistory{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/profile-recalculation/1/history", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("company_id")
	ctx.SetParamValues("1")
	assertStatus(t, controllers.NewAdminProfileRecalculationController(svc).GetHistory, ctx, http.StatusOK)
	svc.AssertExpectations(t)
}

// ===== AdminUserController =====

func TestAdminUserController_List_ServiceError(t *testing.T) {
	repo := &mocks.UserRepositoryMock{}
	repo.On("ListUsersPaged", 25, 0, "").Return(nil, int64(0), errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminUserController(repo, nil).List, newCtx(req, rec), http.StatusInternalServerError)
	repo.AssertExpectations(t)
}

func TestAdminUserController_List_Success(t *testing.T) {
	repo := &mocks.UserRepositoryMock{}
	users := []entity.User{{Email: "user@example.com"}}
	repo.On("ListUsersPaged", 25, 0, "").Return(users, int64(1), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewAdminUserController(repo, nil).List, newCtx(req, rec), http.StatusOK)
	repo.AssertExpectations(t)
}

func TestAdminUserController_Update_InvalidUserID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/abc", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	assertStatus(t, controllers.NewAdminUserController(nil, nil).Update, ctx, http.StatusBadRequest)
}

func TestAdminUserController_Update_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, controllers.NewAdminUserController(nil, nil).Update, ctx, http.StatusBadRequest)
}

func TestAdminUserController_Update_UserNotFound(t *testing.T) {
	repo := &mocks.UserRepositoryMock{}
	repo.On("GetUserByID", uint(1)).Return(nil, errors.New("not found"))

	body, _ := json.Marshal(map[string]bool{"is_admin": true})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, controllers.NewAdminUserController(repo, nil).Update, ctx, http.StatusNotFound)
	repo.AssertExpectations(t)
}

func TestAdminUserController_Update_InvalidTargetLevel(t *testing.T) {
	repo := &mocks.UserRepositoryMock{}
	user := &entity.User{Email: "user@example.com"}
	repo.On("GetUserByID", uint(1)).Return(user, nil)

	targetLevel := "invalid"
	body, _ := json.Marshal(map[string]*string{"target_level": &targetLevel})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, controllers.NewAdminUserController(repo, nil).Update, ctx, http.StatusBadRequest)
	repo.AssertExpectations(t)
}

func TestAdminUserController_Update_Success(t *testing.T) {
	repo := &mocks.UserRepositoryMock{}
	audit := &mocks.AuditLogServiceMock{}
	user := &entity.User{Email: "user@example.com"}
	repo.On("GetUserByID", uint(1)).Return(user, nil)
	repo.On("UpdateUser", mock.Anything).Return(nil)
	audit.On("Record", "", "user.update", "user", uint(0), mock.Anything).Return()

	isAdmin := true
	body, _ := json.Marshal(map[string]*bool{"is_admin": &isAdmin})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, controllers.NewAdminUserController(repo, audit).Update, ctx, http.StatusOK)
	repo.AssertExpectations(t)
	audit.AssertExpectations(t)
}
