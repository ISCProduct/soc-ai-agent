package controllers_test

// ChatControllerのHTTPハンドラーテスト (Issue #434)
//
// 実行: cd Backend && go test ./test/controllers/... -run "Chat" -v

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

func newChatController(
	chatSvc *mocks.ChatServiceMock,
	matchSvc *mocks.MatchingServiceMock,
	analysisSvc *mocks.AnalysisScoringServiceMock,
	userRepo *mocks.UserRepositoryMock,
	emailSvc *mocks.EmailServiceMock,
) *controllers.ChatController {
	return controllers.NewChatController(chatSvc, matchSvc, analysisSvc, userRepo, emailSvc)
}

// ===== GetHistory =====

func TestChatController_GetHistory_Unauthorized(t *testing.T) {
	c := controllers.NewChatController(nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/chat/history?session_id=s1", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, c.GetHistory, newCtx(req, rec), http.StatusUnauthorized)
}

func TestChatController_GetHistory_MissingSessionID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/chat/history", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewChatController(nil, nil, nil, nil, nil).GetHistory, newCtx(req, rec), http.StatusBadRequest)
}

func TestChatController_GetHistory_ServiceError(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	chatSvc.On("GetChatHistory", "s1").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/chat/history?session_id=s1", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).GetHistory, newCtx(req, rec), http.StatusInternalServerError)
	chatSvc.AssertExpectations(t)
}

func TestChatController_GetHistory_Success(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	history := []models.ChatMessage{{SessionID: "s1", Role: "user"}}
	chatSvc.On("GetChatHistory", "s1").Return(history, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/history?session_id=s1", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).GetHistory, newCtx(req, rec), http.StatusOK)
	chatSvc.AssertExpectations(t)
}

func TestChatController_GetHistory_Forbidden(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	// userID=1でリクエストするが、履歴のUserID=2（別ユーザー）
	history := []models.ChatMessage{{UserID: 2, SessionID: "s1", Role: "user"}}
	chatSvc.On("GetChatHistory", "s1").Return(history, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/history?session_id=s1", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).GetHistory, newCtx(req, rec), http.StatusForbidden)
	chatSvc.AssertExpectations(t)
}

// ===== GetScores =====

func TestChatController_GetScores_Unauthorized(t *testing.T) {
	c := controllers.NewChatController(nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/chat/scores?session_id=s1", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, c.GetScores, newCtx(req, rec), http.StatusUnauthorized)
}

func TestChatController_GetScores_MissingSessionID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/chat/scores", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewChatController(nil, nil, nil, nil, nil).GetScores, newCtx(req, rec), http.StatusBadRequest)
}

func TestChatController_GetScores_ServiceError(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	chatSvc.On("GetUserScores", uint(1), "s1").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/chat/scores?session_id=s1", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).GetScores, newCtx(req, rec), http.StatusInternalServerError)
	chatSvc.AssertExpectations(t)
}

func TestChatController_GetScores_Success(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	scores := []entity.UserWeightScore{{UserID: 1}}
	chatSvc.On("GetUserScores", uint(1), "s1").Return(scores, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/scores?session_id=s1", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).GetScores, newCtx(req, rec), http.StatusOK)
	chatSvc.AssertExpectations(t)
}

// ===== ToggleFavorite =====

func TestChatController_ToggleFavorite_Unauthorized(t *testing.T) {
	c := controllers.NewChatController(nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/chat/favorite", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, c.ToggleFavorite, newCtx(req, rec), http.StatusUnauthorized)
}

func TestChatController_ToggleFavorite_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/chat/favorite", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewChatController(nil, nil, nil, nil, nil).ToggleFavorite, newCtx(req, rec), http.StatusBadRequest)
}

func TestChatController_ToggleFavorite_MissingMatchID(t *testing.T) {
	body, _ := json.Marshal(map[string]uint{"match_id": 0})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/favorite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewChatController(nil, nil, nil, nil, nil).ToggleFavorite, newCtx(req, rec), http.StatusBadRequest)
}

func TestChatController_ToggleFavorite_Forbidden(t *testing.T) {
	matchSvc := &mocks.MatchingServiceMock{}
	matchSvc.On("ToggleFavorite", uint(5), uint(1)).Return(services.ErrForbidden)

	body, _ := json.Marshal(map[string]uint{"match_id": 5})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/favorite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(nil, matchSvc, nil, nil, nil).ToggleFavorite, newCtx(req, rec), http.StatusForbidden)
	matchSvc.AssertExpectations(t)
}

func TestChatController_ToggleFavorite_ServiceError(t *testing.T) {
	matchSvc := &mocks.MatchingServiceMock{}
	matchSvc.On("ToggleFavorite", uint(5), uint(1)).Return(errors.New("db error"))

	body, _ := json.Marshal(map[string]uint{"match_id": 5})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/favorite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(nil, matchSvc, nil, nil, nil).ToggleFavorite, newCtx(req, rec), http.StatusInternalServerError)
	matchSvc.AssertExpectations(t)
}

func TestChatController_ToggleFavorite_Success(t *testing.T) {
	matchSvc := &mocks.MatchingServiceMock{}
	matchSvc.On("ToggleFavorite", uint(5), uint(1)).Return(nil)

	body, _ := json.Marshal(map[string]uint{"match_id": 5})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/favorite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(nil, matchSvc, nil, nil, nil).ToggleFavorite, newCtx(req, rec), http.StatusOK)
	matchSvc.AssertExpectations(t)
}

// ===== GetAnalysisSummary =====

func TestChatController_GetAnalysisSummary_Unauthorized(t *testing.T) {
	c := controllers.NewChatController(nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/chat/analysis?session_id=s1", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, c.GetAnalysisSummary, newCtx(req, rec), http.StatusUnauthorized)
}

func TestChatController_GetAnalysisSummary_MissingSessionID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/chat/analysis", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewChatController(nil, nil, nil, nil, nil).GetAnalysisSummary, newCtx(req, rec), http.StatusBadRequest)
}

func TestChatController_GetAnalysisSummary_ServiceUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/chat/analysis?session_id=s1", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	// analysisService=nilを渡す
	assertStatus(t, controllers.NewChatController(nil, nil, nil, nil, nil).GetAnalysisSummary, newCtx(req, rec), http.StatusServiceUnavailable)
}

func TestChatController_GetAnalysisSummary_ServiceError(t *testing.T) {
	analysisSvc := &mocks.AnalysisScoringServiceMock{}
	analysisSvc.On("BuildAnalysisSummary", mock.Anything, uint(1), "s1").Return(nil, errors.New("service error"))

	req := httptest.NewRequest(http.MethodGet, "/api/chat/analysis?session_id=s1", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(nil, nil, analysisSvc, nil, nil).GetAnalysisSummary, newCtx(req, rec), http.StatusInternalServerError)
	analysisSvc.AssertExpectations(t)
}

func TestChatController_GetAnalysisSummary_Success(t *testing.T) {
	analysisSvc := &mocks.AnalysisScoringServiceMock{}
	summary := &services.AnalysisSummary{}
	analysisSvc.On("BuildAnalysisSummary", mock.Anything, uint(1), "s1").Return(summary, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/analysis?session_id=s1", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(nil, nil, analysisSvc, nil, nil).GetAnalysisSummary, newCtx(req, rec), http.StatusOK)
	analysisSvc.AssertExpectations(t)
}

// ===== SendReport =====

func TestChatController_SendReport_Unauthorized(t *testing.T) {
	c := controllers.NewChatController(nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/chat/send-report", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, c.SendReport, newCtx(req, rec), http.StatusUnauthorized)
}

func TestChatController_SendReport_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/chat/send-report", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewChatController(nil, nil, nil, nil, nil).SendReport, newCtx(req, rec), http.StatusBadRequest)
}

func TestChatController_SendReport_MissingSessionID(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"session_id": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/send-report", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewChatController(nil, nil, nil, nil, nil).SendReport, newCtx(req, rec), http.StatusBadRequest)
}

func TestChatController_SendReport_UserNotFound(t *testing.T) {
	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("GetUserByID", uint(1)).Return(nil, errors.New("not found"))

	body, _ := json.Marshal(map[string]string{"session_id": "s1"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/send-report", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(nil, nil, nil, userRepo, nil).SendReport, newCtx(req, rec), http.StatusNotFound)
	userRepo.AssertExpectations(t)
}

func TestChatController_SendReport_GuestForbidden(t *testing.T) {
	userRepo := &mocks.UserRepositoryMock{}
	guestUser := &entity.User{IsGuest: true}
	userRepo.On("GetUserByID", uint(1)).Return(guestUser, nil)

	body, _ := json.Marshal(map[string]string{"session_id": "s1"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/send-report", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(nil, nil, nil, userRepo, nil).SendReport, newCtx(req, rec), http.StatusForbidden)
	userRepo.AssertExpectations(t)
}

func TestChatController_SendReport_AnalysisError(t *testing.T) {
	userRepo := &mocks.UserRepositoryMock{}
	analysisSvc := &mocks.AnalysisScoringServiceMock{}
	user := &entity.User{Email: "test@example.com", IsGuest: false}
	userRepo.On("GetUserByID", uint(1)).Return(user, nil)
	analysisSvc.On("BuildAnalysisSummary", mock.Anything, uint(1), "s1").Return(nil, errors.New("analysis error"))

	body, _ := json.Marshal(map[string]string{"session_id": "s1"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/send-report", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(nil, nil, analysisSvc, userRepo, nil).SendReport, newCtx(req, rec), http.StatusInternalServerError)
	analysisSvc.AssertExpectations(t)
}

func TestChatController_SendReport_Success(t *testing.T) {
	userRepo := &mocks.UserRepositoryMock{}
	analysisSvc := &mocks.AnalysisScoringServiceMock{}
	matchSvc := &mocks.MatchingServiceMock{}
	chatSvc := &mocks.ChatServiceMock{}
	emailSvc := &mocks.EmailServiceMock{}

	user := &entity.User{Email: "test@example.com", IsGuest: false}
	summary := &services.AnalysisSummary{}
	userRepo.On("GetUserByID", uint(1)).Return(user, nil)
	analysisSvc.On("BuildAnalysisSummary", mock.Anything, uint(1), "s1").Return(summary, nil)
	matchSvc.On("GetTopMatches", mock.Anything, uint(1), "s1", 5).Return([]*entity.UserCompanyMatch{}, nil)
	chatSvc.On("GetUserScores", uint(1), "s1").Return([]entity.UserWeightScore{}, nil)
	emailSvc.On("SendAnalysisReport", user, summary, mock.Anything, "s1").Return(nil)

	body, _ := json.Marshal(map[string]string{"session_id": "s1"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/send-report", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(chatSvc, matchSvc, analysisSvc, userRepo, emailSvc).SendReport, newCtx(req, rec), http.StatusOK)
}

// ===== GetSessions =====

func TestChatController_GetSessions_Unauthorized(t *testing.T) {
	c := controllers.NewChatController(nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, c.GetSessions, newCtx(req, rec), http.StatusUnauthorized)
}

func TestChatController_GetSessions_ServiceError(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	chatSvc.On("GetUserChatSessions", uint(1)).Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).GetSessions, newCtx(req, rec), http.StatusInternalServerError)
	chatSvc.AssertExpectations(t)
}

func TestChatController_GetSessions_Success(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	sessions := []models.ChatSession{{SessionID: "s1"}}
	chatSvc.On("GetUserChatSessions", uint(1)).Return(sessions, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).GetSessions, newCtx(req, rec), http.StatusOK)
	chatSvc.AssertExpectations(t)
}

// ===== GetRecommendations =====

func TestChatController_GetRecommendations_Unauthorized(t *testing.T) {
	c := controllers.NewChatController(nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/chat/recommendations?session_id=s1", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, c.GetRecommendations, newCtx(req, rec), http.StatusUnauthorized)
}

func TestChatController_GetRecommendations_MissingSessionID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/chat/recommendations", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewChatController(nil, nil, nil, nil, nil).GetRecommendations, newCtx(req, rec), http.StatusBadRequest)
}

func TestChatController_GetRecommendations_NoMatches_ReturnEmpty(t *testing.T) {
	matchSvc := &mocks.MatchingServiceMock{}
	chatSvc := &mocks.ChatServiceMock{}
	matchSvc.On("GetTopMatches", mock.Anything, uint(1), "s1", 10).Return(nil, nil)
	matchSvc.On("GetDiagnostics", uint(1), "s1").Return(nil, nil)
	chatSvc.On("GetUserScores", uint(1), "s1").Return([]entity.UserWeightScore{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/recommendations?session_id=s1", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(chatSvc, matchSvc, nil, nil, nil).GetRecommendations, newCtx(req, rec), http.StatusOK)
	matchSvc.AssertExpectations(t)
}

func TestChatController_GetRecommendations_WithMatches_Success(t *testing.T) {
	matchSvc := &mocks.MatchingServiceMock{}
	chatSvc := &mocks.ChatServiceMock{}
	matches := []*entity.UserCompanyMatch{
		{MatchScore: 85.0, Company: &entity.Company{ID: 1, Name: "Test Corp"}},
	}
	matchSvc.On("GetTopMatches", mock.Anything, uint(1), "s1", 10).Return(matches, nil)
	chatSvc.On("GetUserScores", uint(1), "s1").Return([]entity.UserWeightScore{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/recommendations?session_id=s1", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newChatController(chatSvc, matchSvc, nil, nil, nil).GetRecommendations, newCtx(req, rec), http.StatusOK)
	matchSvc.AssertExpectations(t)
}
