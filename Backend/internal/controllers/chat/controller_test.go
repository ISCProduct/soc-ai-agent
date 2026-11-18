package chat_test

// ChatControllerのHTTPハンドラーテスト (Issue #434)
//
// 実行: cd Backend && go test ./internal/controllers/chat/... -run "Chat" -v

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	chatcontrollers "Backend/internal/controllers/chat"
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"
	"Backend/internal/services/analysis"
	"Backend/internal/services/chat"
	"Backend/internal/services/matching"
	"Backend/internal/services/shared"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/mock"
)

func newChatController(
	chatSvc *mocks.ChatServiceMock,
	matchSvc *mocks.MatchingServiceMock,
	analysisSvc *mocks.AnalysisScoringServiceMock,
	userRepo *mocks.UserRepositoryMock,
	emailSvc *mocks.EmailServiceMock,
) *chatcontrollers.ChatController {
	return chatcontrollers.NewChatController(chatSvc, matchSvc, analysisSvc, userRepo, emailSvc)
}

// ===== GetHistory =====

func TestChatController_GetHistory_Unauthorized(t *testing.T) {
	c := chatcontrollers.NewChatController(nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/chat/history?session_id=s1", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.GetHistory, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestChatController_GetHistory_MissingSessionID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/chat/history", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, chatcontrollers.NewChatController(nil, nil, nil, nil, nil).GetHistory, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestChatController_GetHistory_ServiceError(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	chatSvc.On("SessionHasOtherUserMessages", "s1", uint(1)).Return(false, nil)
	chatSvc.On("GetChatHistoryForUser", "s1", uint(1)).Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/chat/history?session_id=s1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).GetHistory, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
	chatSvc.AssertExpectations(t)
}

func TestChatController_GetHistory_Success(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	history := []models.ChatMessage{{UserID: 1, SessionID: "s1", Role: "user"}}
	chatSvc.On("SessionHasOtherUserMessages", "s1", uint(1)).Return(false, nil)
	chatSvc.On("GetChatHistoryForUser", "s1", uint(1)).Return(history, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/history?session_id=s1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).GetHistory, testsupport.NewCtx(req, rec), http.StatusOK)
	chatSvc.AssertExpectations(t)
}

func TestChatController_GetHistory_Forbidden(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	// userID=1 でリクエスト。セッションに他人のメッセージがあるので拒否される（#1156）
	chatSvc.On("SessionHasOtherUserMessages", "s1", uint(1)).Return(true, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/history?session_id=s1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).GetHistory, testsupport.NewCtx(req, rec), http.StatusForbidden)
	chatSvc.AssertExpectations(t)
}

// ===== Chat (#946: session_id所有者チェック) =====

func TestChatController_Chat_Forbidden_ExistingSessionOwnedByAnotherUser(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	chatSvc.On("ClaimSessionOwnership", "s1", uint(1)).Return(shared.ErrForbidden)

	body := `{"session_id":"s1","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).Chat, testsupport.NewCtx(req, rec), http.StatusForbidden)
	chatSvc.AssertExpectations(t)
	chatSvc.AssertNotCalled(t, "SessionHasOtherUserMessages", mock.Anything, mock.Anything)
	chatSvc.AssertNotCalled(t, "ProcessChat", mock.Anything, mock.Anything)
}

// user_id が未設定(0)のメッセージが存在するセッションも、スコープ付きクエリでは
// 誰の履歴にも現れないため拒否される（#1156）。
func TestChatController_Chat_Forbidden_ExistingSessionWithUnsetOwner(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	chatSvc.On("ClaimSessionOwnership", "s0", uint(1)).Return(shared.ErrForbidden)

	body := `{"session_id":"s0","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).Chat, testsupport.NewCtx(req, rec), http.StatusForbidden)
	chatSvc.AssertExpectations(t)
	chatSvc.AssertNotCalled(t, "SessionHasOtherUserMessages", mock.Anything, mock.Anything)
	chatSvc.AssertNotCalled(t, "ProcessChat", mock.Anything, mock.Anything)
}

func TestChatController_Chat_Success_NewSession(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	chatSvc.On("ClaimSessionOwnership", "s2", uint(1)).Return(nil)
	chatSvc.On("SessionHasOtherUserMessages", "s2", uint(1)).Return(false, nil)
	chatSvc.On("ProcessChat", mock.Anything, mock.Anything).Return(&chat.ChatResponse{Response: "ok"}, nil)

	body := `{"session_id":"s2","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).Chat, testsupport.NewCtx(req, rec), http.StatusOK)
	chatSvc.AssertExpectations(t)
}

func TestChatController_Chat_Success_OwnExistingSession(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	chatSvc.On("ClaimSessionOwnership", "s3", uint(1)).Return(nil)
	chatSvc.On("SessionHasOtherUserMessages", "s3", uint(1)).Return(false, nil)
	chatSvc.On("ProcessChat", mock.Anything, mock.Anything).Return(&chat.ChatResponse{Response: "ok"}, nil)

	body := `{"session_id":"s3","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).Chat, testsupport.NewCtx(req, rec), http.StatusOK)
	chatSvc.AssertExpectations(t)
	chatSvc.AssertNotCalled(t, "GetChatHistoryForUser", mock.Anything, mock.Anything)
}

// 自分のメッセージがあっても、同じ session_id に他人のメッセージが混在していれば
// 双方に対して拒否する（#1156 フェイルクローズ）。
//
// 「自分のメッセージが1件でもあれば許可」だと混在セッションで両者が通り、
// 下流の要約・埋め込み・分析に相手の自由記述が混ざる。
func TestChatController_Chat_Forbidden_MixedOwnerSession(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	chatSvc.On("ClaimSessionOwnership", "s4", uint(1)).Return(nil)
	chatSvc.On("SessionHasOtherUserMessages", "s4", uint(1)).Return(true, nil)

	body := `{"session_id":"s4","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).Chat, testsupport.NewCtx(req, rec), http.StatusForbidden)
	chatSvc.AssertExpectations(t)
	chatSvc.AssertNotCalled(t, "GetChatHistoryForUser", mock.Anything, mock.Anything)
	chatSvc.AssertNotCalled(t, "ProcessChat", mock.Anything, mock.Anything)
}

// 他人判定のクエリが失敗したら 500 にする（「他人はいない」に倒さない）。
func TestChatController_Chat_InternalError_OwnershipCheckFails(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	chatSvc.On("ClaimSessionOwnership", "s5", uint(1)).Return(nil)
	chatSvc.On("SessionHasOtherUserMessages", "s5", uint(1)).Return(false, errors.New("db error"))

	body := `{"session_id":"s5","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).Chat, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
	chatSvc.AssertExpectations(t)
	chatSvc.AssertNotCalled(t, "ProcessChat", mock.Anything, mock.Anything)
}

// ===== GetScores =====

func TestChatController_GetScores_Unauthorized(t *testing.T) {
	c := chatcontrollers.NewChatController(nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/chat/scores?session_id=s1", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.GetScores, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestChatController_GetScores_MissingSessionID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/chat/scores", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, chatcontrollers.NewChatController(nil, nil, nil, nil, nil).GetScores, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestChatController_GetScores_ServiceError(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	chatSvc.On("GetUserScores", uint(1), "s1").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/chat/scores?session_id=s1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).GetScores, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
	chatSvc.AssertExpectations(t)
}

func TestChatController_GetScores_Success(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	scores := []models.UserWeightScore{{UserID: 1}}
	chatSvc.On("GetUserScores", uint(1), "s1").Return(scores, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/scores?session_id=s1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).GetScores, testsupport.NewCtx(req, rec), http.StatusOK)
	chatSvc.AssertExpectations(t)
}

// 応答のキーは従来どおり Go のフィールド名（api/openapi.yaml の UserWeightScore）。
// models.UserWeightScore の json タグ（id / category …）や Source を出してはいけない。
func TestChatController_GetScores_応答のキーは従来のまま(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	chatSvc.On("GetUserScores", uint(1), "s1").Return([]models.UserWeightScore{
		{ID: 5, UserID: 1, SessionID: "s1", Source: models.ScoreSourceAssessment, WeightCategory: "技術志向", Score: 70},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/scores?session_id=s1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).GetScores, testsupport.NewCtx(req, rec), http.StatusOK)

	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["WeightCategory"] != "技術志向" || rows[0]["Score"] != float64(70) {
		t.Fatalf("応答が従来の形ではない: %s", rec.Body.String())
	}
	for _, k := range []string{"category", "id", "Source", "OrganizationID", "source", "organization_id"} {
		if _, ok := rows[0][k]; ok {
			t.Errorf("出してはいけないキー %s がある: %s", k, rec.Body.String())
		}
	}
}

// ===== ToggleFavorite =====

func TestChatController_ToggleFavorite_Unauthorized(t *testing.T) {
	c := chatcontrollers.NewChatController(nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/chat/favorite", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.ToggleFavorite, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestChatController_ToggleFavorite_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/chat/favorite", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, chatcontrollers.NewChatController(nil, nil, nil, nil, nil).ToggleFavorite, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestChatController_ToggleFavorite_MissingMatchID(t *testing.T) {
	body, _ := json.Marshal(map[string]uint{"match_id": 0})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/favorite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, chatcontrollers.NewChatController(nil, nil, nil, nil, nil).ToggleFavorite, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestChatController_ToggleFavorite_Forbidden(t *testing.T) {
	matchSvc := &mocks.MatchingServiceMock{}
	matchSvc.On("ToggleFavorite", uint(5), uint(1)).Return(shared.ErrForbidden)

	body, _ := json.Marshal(map[string]uint{"match_id": 5})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/favorite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(nil, matchSvc, nil, nil, nil).ToggleFavorite, testsupport.NewCtx(req, rec), http.StatusForbidden)
	matchSvc.AssertExpectations(t)
}

func TestChatController_ToggleFavorite_NotFound(t *testing.T) {
	matchSvc := &mocks.MatchingServiceMock{}
	matchSvc.On("ToggleFavorite", uint(5), uint(1)).Return(shared.ErrNotFound)

	body, _ := json.Marshal(map[string]uint{"match_id": 5})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/favorite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(nil, matchSvc, nil, nil, nil).ToggleFavorite, testsupport.NewCtx(req, rec), http.StatusNotFound)
	matchSvc.AssertExpectations(t)
}

func TestChatController_ToggleFavorite_ServiceError(t *testing.T) {
	matchSvc := &mocks.MatchingServiceMock{}
	matchSvc.On("ToggleFavorite", uint(5), uint(1)).Return(errors.New("db error"))

	body, _ := json.Marshal(map[string]uint{"match_id": 5})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/favorite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(nil, matchSvc, nil, nil, nil).ToggleFavorite, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
	matchSvc.AssertExpectations(t)
}

func TestChatController_ToggleFavorite_Success(t *testing.T) {
	matchSvc := &mocks.MatchingServiceMock{}
	matchSvc.On("ToggleFavorite", uint(5), uint(1)).Return(nil)

	body, _ := json.Marshal(map[string]uint{"match_id": 5})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/favorite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(nil, matchSvc, nil, nil, nil).ToggleFavorite, testsupport.NewCtx(req, rec), http.StatusOK)
	matchSvc.AssertExpectations(t)
}

// ===== GetAnalysisSummary =====

func TestChatController_GetAnalysisSummary_Unauthorized(t *testing.T) {
	c := chatcontrollers.NewChatController(nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/chat/analysis?session_id=s1", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.GetAnalysisSummary, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestChatController_GetAnalysisSummary_MissingSessionID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/chat/analysis", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, chatcontrollers.NewChatController(nil, nil, nil, nil, nil).GetAnalysisSummary, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestChatController_GetAnalysisSummary_ServiceUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/chat/analysis?session_id=s1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	// analysisService=nilを渡す
	testsupport.AssertStatus(t, chatcontrollers.NewChatController(nil, nil, nil, nil, nil).GetAnalysisSummary, testsupport.NewCtx(req, rec), http.StatusServiceUnavailable)
}

func TestChatController_GetAnalysisSummary_ServiceError(t *testing.T) {
	analysisSvc := &mocks.AnalysisScoringServiceMock{}
	analysisSvc.On("BuildAnalysisSummary", mock.Anything, uint(1), "s1").Return(nil, errors.New("service error"))

	req := httptest.NewRequest(http.MethodGet, "/api/chat/analysis?session_id=s1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(nil, nil, analysisSvc, nil, nil).GetAnalysisSummary, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
	analysisSvc.AssertExpectations(t)
}

func TestChatController_GetAnalysisSummary_Success(t *testing.T) {
	analysisSvc := &mocks.AnalysisScoringServiceMock{}
	summary := &analysis.AnalysisSummary{}
	analysisSvc.On("BuildAnalysisSummary", mock.Anything, uint(1), "s1").Return(summary, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/analysis?session_id=s1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(nil, nil, analysisSvc, nil, nil).GetAnalysisSummary, testsupport.NewCtx(req, rec), http.StatusOK)
	analysisSvc.AssertExpectations(t)
}

// ===== SendReport =====

func TestChatController_SendReport_Unauthorized(t *testing.T) {
	c := chatcontrollers.NewChatController(nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/chat/send-report", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.SendReport, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestChatController_SendReport_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/chat/send-report", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, chatcontrollers.NewChatController(nil, nil, nil, nil, nil).SendReport, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestChatController_SendReport_MissingSessionID(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"session_id": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/send-report", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, chatcontrollers.NewChatController(nil, nil, nil, nil, nil).SendReport, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestChatController_SendReport_UserNotFound(t *testing.T) {
	userRepo := &mocks.UserRepositoryMock{}
	userRepo.On("GetUserByID", uint(1)).Return(nil, errors.New("not found"))

	body, _ := json.Marshal(map[string]string{"session_id": "s1"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/send-report", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(nil, nil, nil, userRepo, nil).SendReport, testsupport.NewCtx(req, rec), http.StatusNotFound)
	userRepo.AssertExpectations(t)
}

func TestChatController_SendReport_GuestForbidden(t *testing.T) {
	userRepo := &mocks.UserRepositoryMock{}
	guestUser := &models.User{IsGuest: true}
	userRepo.On("GetUserByID", uint(1)).Return(guestUser, nil)

	body, _ := json.Marshal(map[string]string{"session_id": "s1"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/send-report", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(nil, nil, nil, userRepo, nil).SendReport, testsupport.NewCtx(req, rec), http.StatusForbidden)
	userRepo.AssertExpectations(t)
}

func TestChatController_SendReport_AnalysisError(t *testing.T) {
	userRepo := &mocks.UserRepositoryMock{}
	analysisSvc := &mocks.AnalysisScoringServiceMock{}
	user := &models.User{Email: "test@example.com", IsGuest: false}
	userRepo.On("GetUserByID", uint(1)).Return(user, nil)
	analysisSvc.On("BuildAnalysisSummary", mock.Anything, uint(1), "s1").Return(nil, errors.New("analysis error"))

	body, _ := json.Marshal(map[string]string{"session_id": "s1"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/send-report", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(nil, nil, analysisSvc, userRepo, nil).SendReport, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
	analysisSvc.AssertExpectations(t)
}

func TestChatController_SendReport_Success(t *testing.T) {
	userRepo := &mocks.UserRepositoryMock{}
	analysisSvc := &mocks.AnalysisScoringServiceMock{}
	matchSvc := &mocks.MatchingServiceMock{}
	chatSvc := &mocks.ChatServiceMock{}
	emailSvc := &mocks.EmailServiceMock{}

	user := &models.User{Email: "test@example.com", IsGuest: false}
	summary := &analysis.AnalysisSummary{}
	userRepo.On("GetUserByID", uint(1)).Return(user, nil)
	analysisSvc.On("BuildAnalysisSummary", mock.Anything, uint(1), "s1").Return(summary, nil)
	matchSvc.On("GetTopMatches", mock.Anything, uint(1), "s1", 5).Return([]*models.UserCompanyMatch{}, nil)
	chatSvc.On("GetUserScores", uint(1), "s1").Return([]models.UserWeightScore{}, nil)
	emailSvc.On("SendAnalysisReport", user, summary, mock.Anything, "s1").Return(nil)

	body, _ := json.Marshal(map[string]string{"session_id": "s1"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/send-report", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, matchSvc, analysisSvc, userRepo, emailSvc).SendReport, testsupport.NewCtx(req, rec), http.StatusOK)
}

// ===== GetSessions =====

func TestChatController_GetSessions_Unauthorized(t *testing.T) {
	c := chatcontrollers.NewChatController(nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.GetSessions, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestChatController_GetSessions_ServiceError(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	chatSvc.On("GetUserChatSessions", uint(1)).Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).GetSessions, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
	chatSvc.AssertExpectations(t)
}

func TestChatController_GetSessions_Success(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	sessions := []models.ChatSession{{SessionID: "s1"}}
	chatSvc.On("GetUserChatSessions", uint(1)).Return(sessions, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).GetSessions, testsupport.NewCtx(req, rec), http.StatusOK)
	chatSvc.AssertExpectations(t)
}

// ===== GetRecommendations =====

func TestChatController_GetRecommendations_Unauthorized(t *testing.T) {
	c := chatcontrollers.NewChatController(nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/chat/recommendations?session_id=s1", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.GetRecommendations, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestChatController_GetRecommendations_MissingSessionID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/chat/recommendations", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, chatcontrollers.NewChatController(nil, nil, nil, nil, nil).GetRecommendations, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestChatController_GetRecommendations_NoMatches_ReturnEmpty(t *testing.T) {
	matchSvc := &mocks.MatchingServiceMock{}
	chatSvc := &mocks.ChatServiceMock{}
	matchSvc.On("GetTopMatches", mock.Anything, uint(1), "s1", 10).Return(nil, nil)
	matchSvc.On("GetDiagnostics", uint(1), "s1").Return(nil, nil)
	chatSvc.On("GetUserScores", uint(1), "s1").Return([]models.UserWeightScore{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/recommendations?session_id=s1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, matchSvc, nil, nil, nil).GetRecommendations, testsupport.NewCtx(req, rec), http.StatusOK)
	matchSvc.AssertExpectations(t)
}

// TestChatController_GetRecommendations_EmptyReason は空レスポンスの reason を固定する（#1380）。
//
// insufficient_company_data はフロント（frontend/app/results/utils.ts）で
// 「企業情報を公開するまでお待ちください」と表示される。企業は公開済みで
// プロファイルだけが無いケースにこれを返すと、公開作業では直らない問題に
// 誤った復旧手順を案内することになる。
func TestChatController_GetRecommendations_EmptyReason(t *testing.T) {
	tests := []struct {
		name string
		diag *matching.MatchingDiagnostics
		want string
	}{
		{
			name: "ユーザースコアが無い",
			diag: &matching.MatchingDiagnostics{UserScoreCount: 0, ActiveCompanyCount: 10},
			want: "insufficient_user_scores",
		},
		{
			name: "公開企業が0社",
			diag: &matching.MatchingDiagnostics{UserScoreCount: 10, ActiveCompanyCount: 0},
			want: "insufficient_company_data",
		},
		{
			name: "公開企業はあるが全社プロファイル未設定",
			diag: &matching.MatchingDiagnostics{
				UserScoreCount: 10, ActiveCompanyCount: 10, CompaniesWithoutProfile: 10,
			},
			want: "insufficient_company_profiles",
		},
		{
			name: "一部だけプロファイル欠損（原因は別）",
			diag: &matching.MatchingDiagnostics{
				UserScoreCount: 10, ActiveCompanyCount: 10, WeightProfileCount: 9, CompaniesWithoutProfile: 1,
			},
			want: "matching_results_empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matchSvc := &mocks.MatchingServiceMock{}
			chatSvc := &mocks.ChatServiceMock{}
			matchSvc.On("GetTopMatches", mock.Anything, uint(1), "s1", 10).Return(nil, nil)
			matchSvc.On("GetDiagnostics", uint(1), "s1").Return(tt.diag, nil)
			chatSvc.On("GetUserScores", uint(1), "s1").Return([]models.UserWeightScore{}, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/chat/recommendations?session_id=s1", nil)
			req = testsupport.WithUserID(req, 1)
			rec := httptest.NewRecorder()
			testsupport.AssertStatus(t, newChatController(chatSvc, matchSvc, nil, nil, nil).GetRecommendations,
				testsupport.NewCtx(req, rec), http.StatusOK)

			var body struct {
				Reason string `json:"reason"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("レスポンスをデコードできない: %v (%s)", err, rec.Body.String())
			}
			if body.Reason != tt.want {
				t.Errorf("reason=%q want %q", body.Reason, tt.want)
			}
			matchSvc.AssertExpectations(t)
		})
	}
}

func TestChatController_GetRecommendations_WithMatches_Success(t *testing.T) {
	matchSvc := &mocks.MatchingServiceMock{}
	chatSvc := &mocks.ChatServiceMock{}
	matches := []*models.UserCompanyMatch{
		{MatchScore: 85.0, Company: models.Company{ID: 1, Name: "Test Corp"}},
	}
	matchSvc.On("GetTopMatches", mock.Anything, uint(1), "s1", 10).Return(matches, nil)
	chatSvc.On("GetUserScores", uint(1), "s1").Return([]models.UserWeightScore{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/recommendations?session_id=s1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, matchSvc, nil, nil, nil).GetRecommendations, testsupport.NewCtx(req, rec), http.StatusOK)
	matchSvc.AssertExpectations(t)
}

// TestChatController_GetRecommendations_LimitCap は limit クエリの頭打ちを固定する(#1478)。
//
// 上限が無いと limit=1000000 がそのままマッチングサービスへ渡り、1リクエストで
// 全件が読まれてJSON化される。他の一覧APIと同じ 100 に揃える。
func TestChatController_GetRecommendations_LimitCap(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantLimit int
	}{
		{name: "未指定は既定の10", query: "", wantLimit: 10},
		{name: "上限内はそのまま", query: "&limit=30", wantLimit: 30},
		{name: "上限ちょうど", query: "&limit=100", wantLimit: 100},
		{name: "上限超過は100へ頭打ち", query: "&limit=101", wantLimit: 100},
		{name: "極端な値も100へ頭打ち", query: "&limit=1000000", wantLimit: 100},
		{name: "0は既定の10", query: "&limit=0", wantLimit: 10},
		{name: "負値は既定の10", query: "&limit=-1", wantLimit: 10},
		{name: "数値以外は既定の10", query: "&limit=abc", wantLimit: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matchSvc := &mocks.MatchingServiceMock{}
			chatSvc := &mocks.ChatServiceMock{}
			matches := []*models.UserCompanyMatch{
				{MatchScore: 85.0, Company: models.Company{ID: 1, Name: "Test Corp"}},
			}
			matchSvc.On("GetTopMatches", mock.Anything, uint(1), "s1", tt.wantLimit).Return(matches, nil)
			chatSvc.On("GetUserScores", uint(1), "s1").Return([]models.UserWeightScore{}, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/chat/recommendations?session_id=s1"+tt.query, nil)
			req = testsupport.WithUserID(req, 1)
			rec := httptest.NewRecorder()
			testsupport.AssertStatus(t, newChatController(chatSvc, matchSvc, nil, nil, nil).GetRecommendations, testsupport.NewCtx(req, rec), http.StatusOK)
			matchSvc.AssertExpectations(t)
		})
	}
}

// --- 推薦への「合う／合わない」（SOCAIAGENT-670） ---

type fakeMatchFeedback struct {
	err       error
	submitted []string
	verdicts  map[uint]string
}

func (f *fakeMatchFeedback) Submit(userID, matchID uint, verdict string) error {
	f.submitted = append(f.submitted, fmt.Sprintf("%d/%d/%s", userID, matchID, verdict))
	return f.err
}
func (f *fakeMatchFeedback) Verdicts(uint, []uint) map[uint]string { return f.verdicts }

func postFeedback(t *testing.T, c *chatcontrollers.ChatController, body string, userID uint) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/match-feedback", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if userID != 0 {
		req = testsupport.WithUserID(req, userID)
	}
	rec := httptest.NewRecorder()
	// ハンドラは、失敗を echo.HTTPError で返す（レコーダーにはステータスが入らない）
	if err := c.SubmitMatchFeedback(testsupport.NewCtx(req, rec)); err != nil {
		var he *echo.HTTPError
		if errors.As(err, &he) {
			return he.Code
		}
		return http.StatusInternalServerError
	}
	return rec.Code
}

func TestChatController_SubmitMatchFeedback(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		user   uint
		err    error
		want   int
		nilFB  bool
		called bool
	}{
		{"成功", `{"match_id":5,"verdict":"fit"}`, 7, nil, http.StatusOK, false, true},
		{"取り消し", `{"match_id":5,"verdict":"none"}`, 7, nil, http.StatusOK, false, true},
		{"未認証", `{"match_id":5,"verdict":"fit"}`, 0, nil, http.StatusUnauthorized, false, false},
		{"match_id が無い", `{"verdict":"fit"}`, 7, nil, http.StatusBadRequest, false, false},
		{"verdict が無い", `{"match_id":5}`, 7, nil, http.StatusBadRequest, false, false},
		{"知らない判定", `{"match_id":5,"verdict":"x"}`, 7, matching.ErrInvalidFeedbackVerdict, http.StatusBadRequest, false, true},
		{"他人の推薦", `{"match_id":5,"verdict":"fit"}`, 7, shared.ErrForbidden, http.StatusForbidden, false, true},
		{"推薦が無い", `{"match_id":5,"verdict":"fit"}`, 7, shared.ErrNotFound, http.StatusNotFound, false, true},
		{"保存に失敗", `{"match_id":5,"verdict":"fit"}`, 7, errors.New("db"), http.StatusInternalServerError, false, true},
		{"未注入", `{"match_id":5,"verdict":"fit"}`, 7, nil, http.StatusServiceUnavailable, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newChatController(&mocks.ChatServiceMock{}, &mocks.MatchingServiceMock{}, nil, nil, nil)
			fb := &fakeMatchFeedback{err: tt.err}
			if !tt.nilFB {
				c.SetMatchFeedback(fb)
			}
			if code := postFeedback(t, c, tt.body, tt.user); code != tt.want {
				t.Fatalf("status=%d, want %d", code, tt.want)
			}
			if (len(fb.submitted) > 0) != tt.called {
				t.Fatalf("Submit の呼び出し: %v", fb.submitted)
			}
		})
	}
}

// 推薦に、本人の「合う／合わない」が付く（SOCAIAGENT-670）。読めない判定は空のまま。
func TestChatController_GetRecommendations_AttachesFeedback(t *testing.T) {
	matchSvc := &mocks.MatchingServiceMock{}
	chatSvc := &mocks.ChatServiceMock{}
	matches := []*models.UserCompanyMatch{
		{ID: 11, UserID: 1, Company: models.Company{ID: 1, Name: "A社"}},
		{ID: 12, UserID: 1, Company: models.Company{ID: 2, Name: "B社"}},
	}
	matchSvc.On("GetTopMatches", mock.Anything, uint(1), "s1", 10).Return(matches, nil)
	matchSvc.On("ReasonScores", uint(1), mock.Anything).Return([]models.UserWeightScore{})
	chatSvc.On("GetUserScores", uint(1), "s1").Return([]models.UserWeightScore{}, nil)

	c := newChatController(chatSvc, matchSvc, nil, nil, nil)
	c.SetMatchFeedback(&fakeMatchFeedback{verdicts: map[uint]string{11: "unfit"}})

	req := httptest.NewRequest(http.MethodGet, "/api/chat/recommendations?session_id=s1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.GetRecommendations, testsupport.NewCtx(req, rec), http.StatusOK)

	var resp matching.RecommendationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Recommendations) != 2 || resp.Recommendations[0].Feedback != "unfit" || resp.Recommendations[1].Feedback != "" {
		t.Fatalf("判定が付くのは回答した推薦だけ: %+v", resp.Recommendations)
	}
}
