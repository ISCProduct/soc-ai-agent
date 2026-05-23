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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ===== AdminAuditController =====

func TestAdminAuditController_List_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/audit-logs", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminAuditController(nil).List(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminAuditController_List_ServiceError(t *testing.T) {
	svc := &mocks.AuditLogServiceMock{}
	svc.On("List", 50).Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/audit-logs", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminAuditController(svc).List(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminAuditController_List_Success(t *testing.T) {
	svc := &mocks.AuditLogServiceMock{}
	logs := []models.AuditLog{{ActorEmail: "admin@example.com", Action: "user.update"}}
	svc.On("List", 50).Return(logs, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/audit-logs", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminAuditController(svc).List(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminAuditController_List_CustomLimit(t *testing.T) {
	svc := &mocks.AuditLogServiceMock{}
	svc.On("List", 10).Return([]models.AuditLog{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/audit-logs?limit=10", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminAuditController(svc).List(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ===== AdminScraperSessionController =====

func TestAdminScraperSessionController_Sessions_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/admin/scraper-sessions", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminScraperSessionController(nil).Sessions(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminScraperSessionController_Sessions_List_ServiceError(t *testing.T) {
	svc := &mocks.ScraperSessionServiceMock{}
	svc.On("List").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/scraper-sessions", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminScraperSessionController(svc).Sessions(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminScraperSessionController_Sessions_List_Success(t *testing.T) {
	svc := &mocks.ScraperSessionServiceMock{}
	svc.On("List").Return([]models.ScraperSession{{SiteKey: "mynavi"}}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/scraper-sessions", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminScraperSessionController(svc).Sessions(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminScraperSessionController_Sessions_Upsert_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/scraper-sessions", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	controllers.NewAdminScraperSessionController(nil).Sessions(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminScraperSessionController_Sessions_Upsert_InvalidExpiresAt(t *testing.T) {
	expiresAt := "not-a-date"
	body, _ := json.Marshal(map[string]interface{}{
		"site_key":   "mynavi",
		"cookies":    "session=abc",
		"expires_at": expiresAt,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/scraper-sessions", bytes.NewReader(body))
	w := httptest.NewRecorder()
	controllers.NewAdminScraperSessionController(nil).Sessions(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
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
	w := httptest.NewRecorder()
	controllers.NewAdminScraperSessionController(svc).Sessions(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminScraperSessionController_SessionDetail_MissingSiteKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/scraper-sessions/", nil)
	req.URL.Path = "/api/admin/scraper-sessions/"
	w := httptest.NewRecorder()
	controllers.NewAdminScraperSessionController(nil).SessionDetail(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminScraperSessionController_SessionDetail_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/scraper-sessions/mynavi", nil)
	req.URL.Path = "/api/admin/scraper-sessions/mynavi"
	w := httptest.NewRecorder()
	controllers.NewAdminScraperSessionController(nil).SessionDetail(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminScraperSessionController_SessionDetail_ServiceError(t *testing.T) {
	svc := &mocks.ScraperSessionServiceMock{}
	svc.On("Delete", "mynavi").Return(errors.New("not found"))

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/scraper-sessions/mynavi", nil)
	req.URL.Path = "/api/admin/scraper-sessions/mynavi"
	w := httptest.NewRecorder()
	controllers.NewAdminScraperSessionController(svc).SessionDetail(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminScraperSessionController_SessionDetail_Success(t *testing.T) {
	svc := &mocks.ScraperSessionServiceMock{}
	svc.On("Delete", "mynavi").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/scraper-sessions/mynavi", nil)
	req.URL.Path = "/api/admin/scraper-sessions/mynavi"
	w := httptest.NewRecorder()
	controllers.NewAdminScraperSessionController(svc).SessionDetail(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

// ===== AdminScoreValidationController =====

func TestAdminScoreValidationController_Route_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/score-validation/unknown", nil)
	req.URL.Path = "/api/admin/score-validation/unknown"
	w := httptest.NewRecorder()
	controllers.NewAdminScoreValidationController(nil).Route(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAdminScoreValidationController_GetCorrelation_Success(t *testing.T) {
	svc := &mocks.ScoreValidationServiceMock{}
	svc.On("GetCorrelationReport").Return(&services.CorrelationReport{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/score-validation/correlation", nil)
	req.URL.Path = "/api/admin/score-validation/correlation"
	w := httptest.NewRecorder()
	controllers.NewAdminScoreValidationController(svc).Route(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminScoreValidationController_GetCorrelation_Error(t *testing.T) {
	svc := &mocks.ScoreValidationServiceMock{}
	svc.On("GetCorrelationReport").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/score-validation/correlation", nil)
	req.URL.Path = "/api/admin/score-validation/correlation"
	w := httptest.NewRecorder()
	controllers.NewAdminScoreValidationController(svc).Route(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminScoreValidationController_GetCalibration_Success(t *testing.T) {
	svc := &mocks.ScoreValidationServiceMock{}
	svc.On("GetCurrentCalibration").Return([]models.ScoreCalibrationWeight{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/score-validation/calibration", nil)
	req.URL.Path = "/api/admin/score-validation/calibration"
	w := httptest.NewRecorder()
	controllers.NewAdminScoreValidationController(svc).Route(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminScoreValidationController_RunCalibration_Success(t *testing.T) {
	svc := &mocks.ScoreValidationServiceMock{}
	svc.On("RunCalibration").Return(&services.CalibrationResult{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/score-validation/calibration/run", nil)
	req.URL.Path = "/api/admin/score-validation/calibration/run"
	w := httptest.NewRecorder()
	controllers.NewAdminScoreValidationController(svc).Route(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminScoreValidationController_ListVariants_Success(t *testing.T) {
	svc := &mocks.ScoreValidationServiceMock{}
	svc.On("ListExperiments").Return([]string{"exp-a", "exp-b"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/score-validation/variants", nil)
	req.URL.Path = "/api/admin/score-validation/variants"
	w := httptest.NewRecorder()
	controllers.NewAdminScoreValidationController(svc).Route(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminScoreValidationController_CreateVariant_MissingFields(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"experiment_name": "exp-a"})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/score-validation/variants", bytes.NewReader(body))
	req.URL.Path = "/api/admin/score-validation/variants"
	w := httptest.NewRecorder()
	controllers.NewAdminScoreValidationController(nil).Route(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
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
	req.URL.Path = "/api/admin/score-validation/variants"
	w := httptest.NewRecorder()
	controllers.NewAdminScoreValidationController(svc).Route(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminScoreValidationController_GetVariantResults_MissingExperiment(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/score-validation/variants/results", nil)
	req.URL.Path = "/api/admin/score-validation/variants/results"
	w := httptest.NewRecorder()
	controllers.NewAdminScoreValidationController(nil).Route(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ===== AdminProfileRecalculationController =====

func TestAdminProfileRecalculationController_Route_InvalidCompanyID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/profile-recalculation/abc", nil)
	req.URL.Path = "/api/admin/profile-recalculation/abc"
	w := httptest.NewRecorder()
	controllers.NewAdminProfileRecalculationController(nil).Route(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminProfileRecalculationController_Route_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/profile-recalculation/1", nil)
	req.URL.Path = "/api/admin/profile-recalculation/1"
	w := httptest.NewRecorder()
	controllers.NewAdminProfileRecalculationController(nil).Route(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAdminProfileRecalculationController_RecalculateAll_Success(t *testing.T) {
	svc := &mocks.ProfileRecalculationServiceMock{}
	svc.On("RecalculateAll", 0).Return([]*services.RecalculationResult{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/profile-recalculation", nil)
	req.URL.Path = "/api/admin/profile-recalculation"
	w := httptest.NewRecorder()
	controllers.NewAdminProfileRecalculationController(svc).Route(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminProfileRecalculationController_RecalculateOne_Success(t *testing.T) {
	svc := &mocks.ProfileRecalculationServiceMock{}
	svc.On("RecalculateCompany", uint(1), 0).Return(&services.RecalculationResult{CompanyID: 1}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/profile-recalculation/1", nil)
	req.URL.Path = "/api/admin/profile-recalculation/1"
	w := httptest.NewRecorder()
	controllers.NewAdminProfileRecalculationController(svc).Route(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminProfileRecalculationController_Rollback_Success(t *testing.T) {
	svc := &mocks.ProfileRecalculationServiceMock{}
	svc.On("Rollback", uint(1)).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/profile-recalculation/1/rollback", nil)
	req.URL.Path = "/api/admin/profile-recalculation/1/rollback"
	w := httptest.NewRecorder()
	controllers.NewAdminProfileRecalculationController(svc).Route(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminProfileRecalculationController_GetHistory_Success(t *testing.T) {
	svc := &mocks.ProfileRecalculationServiceMock{}
	svc.On("GetHistory", uint(1)).Return([]*models.CompanyProfileUpdateHistory{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/profile-recalculation/1/history", nil)
	req.URL.Path = "/api/admin/profile-recalculation/1/history"
	w := httptest.NewRecorder()
	controllers.NewAdminProfileRecalculationController(svc).Route(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ===== AdminUserController =====

func TestAdminUserController_List_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminUserController(nil, nil).List(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminUserController_List_ServiceError(t *testing.T) {
	repo := &mocks.UserRepositoryMock{}
	repo.On("ListUsersPaged", 25, 0, "").Return(nil, int64(0), errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminUserController(repo, nil).List(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	repo.AssertExpectations(t)
}

func TestAdminUserController_List_Success(t *testing.T) {
	repo := &mocks.UserRepositoryMock{}
	users := []entity.User{{Email: "user@example.com"}}
	repo.On("ListUsersPaged", 25, 0, "").Return(users, int64(1), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminUserController(repo, nil).List(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestAdminUserController_Update_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users/1", nil)
	req.URL.Path = "/api/admin/users/1"
	w := httptest.NewRecorder()
	controllers.NewAdminUserController(nil, nil).Update(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminUserController_Update_InvalidUserID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/abc", nil)
	req.URL.Path = "/api/admin/users/abc"
	w := httptest.NewRecorder()
	controllers.NewAdminUserController(nil, nil).Update(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminUserController_Update_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewBufferString("not-json"))
	req.URL.Path = "/api/admin/users/1"
	w := httptest.NewRecorder()
	controllers.NewAdminUserController(nil, nil).Update(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminUserController_Update_UserNotFound(t *testing.T) {
	repo := &mocks.UserRepositoryMock{}
	repo.On("GetUserByID", uint(1)).Return(nil, errors.New("not found"))

	body, _ := json.Marshal(map[string]bool{"is_admin": true})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.URL.Path = "/api/admin/users/1"
	w := httptest.NewRecorder()
	controllers.NewAdminUserController(repo, nil).Update(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestAdminUserController_Update_InvalidTargetLevel(t *testing.T) {
	repo := &mocks.UserRepositoryMock{}
	user := &entity.User{Email: "user@example.com"}
	repo.On("GetUserByID", uint(1)).Return(user, nil)

	targetLevel := "invalid"
	body, _ := json.Marshal(map[string]*string{"target_level": &targetLevel})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.URL.Path = "/api/admin/users/1"
	w := httptest.NewRecorder()
	controllers.NewAdminUserController(repo, nil).Update(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
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
	req.URL.Path = "/api/admin/users/1"
	w := httptest.NewRecorder()
	controllers.NewAdminUserController(repo, audit).Update(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
	audit.AssertExpectations(t)
}
