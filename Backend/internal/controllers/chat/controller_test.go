package chat_test

// ChatControllerのHTTPハンドラーテスト (Issue #434)
//
// 実行: cd Backend && go test ./internal/controllers/chat/... -run "Chat" -v

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/domain/entity"
	chatcontrollers "Backend/internal/controllers/chat"
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"
	"Backend/internal/services/analysis"
	"Backend/internal/services/chat"
	"Backend/internal/services/matching"
	"Backend/internal/services/shared"

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
	chatSvc.On("SessionHasOtherUserMessages", "s1", uint(1)).Return(true, nil)

	body := `{"session_id":"s1","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).Chat, testsupport.NewCtx(req, rec), http.StatusForbidden)
	chatSvc.AssertExpectations(t)
	chatSvc.AssertNotCalled(t, "ProcessChat", mock.Anything, mock.Anything)
}

// user_id が未設定(0)のメッセージが存在するセッションも、スコープ付きクエリでは
// 誰の履歴にも現れないため拒否される（#1156）。
func TestChatController_Chat_Forbidden_ExistingSessionWithUnsetOwner(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	chatSvc.On("SessionHasOtherUserMessages", "s0", uint(1)).Return(true, nil)

	body := `{"session_id":"s0","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).Chat, testsupport.NewCtx(req, rec), http.StatusForbidden)
	chatSvc.AssertExpectations(t)
	chatSvc.AssertNotCalled(t, "ProcessChat", mock.Anything, mock.Anything)
}

func TestChatController_Chat_Success_NewSession(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
	// session s2 はまだメッセージが存在しない新規セッション。
	// Chat は履歴を使わないので所有者判定だけを行う（GetChatHistoryForUser は呼ばない）
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
	// session s3 は既にuserID=1（リクエスト本人）のメッセージのみが存在する
	chatSvc.On("SessionHasOtherUserMessages", "s3", uint(1)).Return(false, nil)
	chatSvc.On("ProcessChat", mock.Anything, mock.Anything).Return(&chat.ChatResponse{Response: "ok"}, nil)

	body := `{"session_id":"s3","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).Chat, testsupport.NewCtx(req, rec), http.StatusOK)
	chatSvc.AssertExpectations(t)
	// 履歴は ProcessChat が LIMIT 付きで読み直すので、ここで全件 SELECT を出さない
	chatSvc.AssertNotCalled(t, "GetChatHistoryForUser", mock.Anything, mock.Anything)
}

// 自分のメッセージがあっても、同じ session_id に他人のメッセージが混在していれば
// 双方に対して拒否する（#1156 フェイルクローズ）。
//
// 「自分のメッセージが1件でもあれば許可」だと混在セッションで両者が通り、
// 下流の要約・埋め込み・分析に相手の自由記述が混ざる。
func TestChatController_Chat_Forbidden_MixedOwnerSession(t *testing.T) {
	chatSvc := &mocks.ChatServiceMock{}
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
	scores := []entity.UserWeightScore{{UserID: 1}}
	chatSvc.On("GetUserScores", uint(1), "s1").Return(scores, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/scores?session_id=s1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, nil, nil, nil, nil).GetScores, testsupport.NewCtx(req, rec), http.StatusOK)
	chatSvc.AssertExpectations(t)
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
	guestUser := &entity.User{IsGuest: true}
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
	user := &entity.User{Email: "test@example.com", IsGuest: false}
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

	user := &entity.User{Email: "test@example.com", IsGuest: false}
	summary := &analysis.AnalysisSummary{}
	userRepo.On("GetUserByID", uint(1)).Return(user, nil)
	analysisSvc.On("BuildAnalysisSummary", mock.Anything, uint(1), "s1").Return(summary, nil)
	matchSvc.On("GetTopMatches", mock.Anything, uint(1), "s1", 5).Return([]*entity.UserCompanyMatch{}, nil)
	chatSvc.On("GetUserScores", uint(1), "s1").Return([]entity.UserWeightScore{}, nil)
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
	chatSvc.On("GetUserScores", uint(1), "s1").Return([]entity.UserWeightScore{}, nil)

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
			chatSvc.On("GetUserScores", uint(1), "s1").Return([]entity.UserWeightScore{}, nil)

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
	matches := []*entity.UserCompanyMatch{
		{MatchScore: 85.0, Company: &entity.Company{ID: 1, Name: "Test Corp"}},
	}
	matchSvc.On("GetTopMatches", mock.Anything, uint(1), "s1", 10).Return(matches, nil)
	chatSvc.On("GetUserScores", uint(1), "s1").Return([]entity.UserWeightScore{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/recommendations?session_id=s1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newChatController(chatSvc, matchSvc, nil, nil, nil).GetRecommendations, testsupport.NewCtx(req, rec), http.StatusOK)
	matchSvc.AssertExpectations(t)
}
