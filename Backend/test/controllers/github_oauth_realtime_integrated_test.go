package controllers_test

// GitHub・OAuth・Realtime・IntegratedProfileコントローラーのHTTPハンドラーテスト
//
// 実行: cd Backend && go test ./test/controllers/... -run "GitHub|OAuth|Realtime|Integrated" -v

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	"Backend/internal/models"
	"Backend/internal/services"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
	tmock "github.com/stretchr/testify/mock"
)

// ========== GitHubController ==========

func newGitHubController(gh *mocks.GitHubServiceMock, ss *mocks.SkillScoreServiceMock) *controllers.GitHubController {
	return controllers.NewGitHubController(gh, ss)
}

// ---- GetProfile ----

func TestGitHubController_GetProfile_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/github/profile", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newGitHubController(nil, nil).GetProfile, newCtx(req, rec), http.StatusUnauthorized)
}

func TestGitHubController_GetProfile_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/github/profile", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()

	gh := &mocks.GitHubServiceMock{}
	gh.On("GetProfile", uint(1)).Return(nil, nil)
	assertStatus(t, newGitHubController(gh, nil).GetProfile, newCtx(req, rec), http.StatusNotFound)
	gh.AssertExpectations(t)
}

func TestGitHubController_GetProfile_ServiceError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/github/profile", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()

	gh := &mocks.GitHubServiceMock{}
	gh.On("GetProfile", uint(1)).Return(nil, errors.New("db error"))
	assertStatus(t, newGitHubController(gh, nil).GetProfile, newCtx(req, rec), http.StatusInternalServerError)
}

func TestGitHubController_GetProfile_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/github/profile", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()

	gh := &mocks.GitHubServiceMock{}
	gh.On("GetProfile", uint(1)).Return(&models.GitHubProfile{UserID: 1, GitHubLogin: "testuser"}, nil)
	gh.On("GetRepositories", uint(1)).Return([]models.GitHubRepo{}, nil)
	gh.On("GetLanguageStats", uint(1)).Return([]models.GitHubLanguageStat{}, nil)
	assertStatus(t, newGitHubController(gh, nil).GetProfile, newCtx(req, rec), http.StatusOK)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body, "profile")
	assert.Contains(t, body, "repositories")
	assert.Contains(t, body, "language_stats")
	gh.AssertExpectations(t)
}

// ---- Sync ----

func TestGitHubController_Sync_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/github/sync", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newGitHubController(nil, nil).Sync, newCtx(req, rec), http.StatusUnauthorized)
}

func TestGitHubController_Sync_ProfileNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/github/sync", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()

	gh := &mocks.GitHubServiceMock{}
	gh.On("GetProfile", uint(1)).Return(nil, nil)
	assertStatus(t, newGitHubController(gh, nil).Sync, newCtx(req, rec), http.StatusNotFound)
}

func TestGitHubController_Sync_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/github/sync", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()

	gh := &mocks.GitHubServiceMock{}
	gh.On("GetProfile", uint(1)).Return(&models.GitHubProfile{UserID: 1}, nil)
	gh.On("TriggerAsyncSync", uint(1), false).Return()
	assertStatus(t, newGitHubController(gh, nil).Sync, newCtx(req, rec), http.StatusOK)
	gh.AssertExpectations(t)
}

// ---- SyncAndWait ----

func TestGitHubController_SyncAndWait_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/github/sync/wait", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newGitHubController(nil, nil).SyncAndWait, newCtx(req, rec), http.StatusUnauthorized)
}

func TestGitHubController_SyncAndWait_ServiceError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/github/sync/wait", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()

	gh := &mocks.GitHubServiceMock{}
	gh.On("SyncUserData", tmock.Anything, uint(1), true).Return(errors.New("network error"))
	assertStatus(t, newGitHubController(gh, nil).SyncAndWait, newCtx(req, rec), http.StatusInternalServerError)
}

func TestGitHubController_SyncAndWait_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/github/sync/wait", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()

	gh := &mocks.GitHubServiceMock{}
	gh.On("SyncUserData", tmock.Anything, uint(1), true).Return(nil)
	assertStatus(t, newGitHubController(gh, nil).SyncAndWait, newCtx(req, rec), http.StatusOK)
	gh.AssertExpectations(t)
}

// ---- GetSkills ----

func TestGitHubController_GetSkills_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/github/skills", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newGitHubController(nil, nil).GetSkills, newCtx(req, rec), http.StatusUnauthorized)
}

func TestGitHubController_GetSkills_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/github/skills", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()

	ss := &mocks.SkillScoreServiceMock{}
	ss.On("GetScores", uint(1)).Return([]models.SkillScore{{UserID: 1}}, nil)
	assertStatus(t, newGitHubController(nil, ss).GetSkills, newCtx(req, rec), http.StatusOK)
	ss.AssertExpectations(t)
}

func TestGitHubController_GetSkills_ServiceError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/github/skills", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()

	ss := &mocks.SkillScoreServiceMock{}
	ss.On("GetScores", uint(1)).Return([]models.SkillScore{}, errors.New("db error"))
	assertStatus(t, newGitHubController(nil, ss).GetSkills, newCtx(req, rec), http.StatusInternalServerError)
}

// ---- ListRepoSummaries ----

func TestGitHubController_ListRepoSummaries_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/github/repo/summaries", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newGitHubController(nil, nil).ListRepoSummaries, newCtx(req, rec), http.StatusUnauthorized)
}

func TestGitHubController_ListRepoSummaries_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/github/repo/summaries", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()

	gh := &mocks.GitHubServiceMock{}
	gh.On("ListRepoSummaries", uint(1)).Return([]models.GitHubRepoSummary{{UserID: 1}}, nil)
	assertStatus(t, newGitHubController(gh, nil).ListRepoSummaries, newCtx(req, rec), http.StatusOK)
	gh.AssertExpectations(t)
}

// ---- SummarizeRepo ----

func TestGitHubController_SummarizeRepo_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/github/repo/summarize", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newGitHubController(nil, nil).SummarizeRepo, newCtx(req, rec), http.StatusUnauthorized)
}

func TestGitHubController_SummarizeRepo_MissingFullName(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"full_name": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/github/repo/summarize", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newGitHubController(nil, nil).SummarizeRepo, newCtx(req, rec), http.StatusBadRequest)
}

func TestGitHubController_SummarizeRepo_Success(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"full_name": "owner/repo", "force_refresh": false})
	req := httptest.NewRequest(http.MethodPost, "/api/github/repo/summarize", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()

	gh := &mocks.GitHubServiceMock{}
	gh.On("SummarizeRepo", tmock.Anything, uint(1), "owner/repo", false, "").
		Return(&models.GitHubRepoSummary{FullName: "owner/repo"}, nil)
	assertStatus(t, newGitHubController(gh, nil).SummarizeRepo, newCtx(req, rec), http.StatusOK)
	gh.AssertExpectations(t)
}

// ========== OAuthController ==========

func newOAuthController(svc *mocks.OAuthServiceMock) *controllers.OAuthController {
	return controllers.NewOAuthController(svc)
}

// ---- GoogleLogin ----

func TestOAuthController_GoogleLogin_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/google", nil)
	rec := httptest.NewRecorder()

	svc := &mocks.OAuthServiceMock{}
	svc.On("GetGoogleAuthURL", tmock.AnythingOfType("string")).Return("https://accounts.google.com/auth?state=xxx")
	assertStatus(t, newOAuthController(svc).GoogleLogin, newCtx(req, rec), http.StatusTemporaryRedirect)
	assert.Contains(t, rec.Header().Get("Location"), "accounts.google.com")
}

// ---- GitHubLogin ----

func TestOAuthController_GitHubLogin_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/github", nil)
	rec := httptest.NewRecorder()

	svc := &mocks.OAuthServiceMock{}
	svc.On("GetGitHubAuthURL", tmock.AnythingOfType("string")).Return("https://github.com/login/oauth/authorize?state=xxx")
	assertStatus(t, newOAuthController(svc).GitHubLogin, newCtx(req, rec), http.StatusTemporaryRedirect)
	assert.Contains(t, rec.Header().Get("Location"), "github.com")
}

// ---- GoogleCallback ----

func TestOAuthController_GoogleCallback_MissingCode(t *testing.T) {
	// state検証をスキップするためcookieなしリクエスト → stateミスマッチでリダイレクト(307)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback", nil)
	rec := httptest.NewRecorder()
	// VerifyOAuthState は cookie がないため false を返す → Redirect 307
	assertStatus(t, newOAuthController(nil).GoogleCallback, newCtx(req, rec), http.StatusTemporaryRedirect)
}

func TestOAuthController_GitHubCallback_MissingCode(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/github/callback", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newOAuthController(nil).GitHubCallback, newCtx(req, rec), http.StatusTemporaryRedirect)
}

// ========== RealtimeController ==========

func newRealtimeController(interviewSvc *mocks.InterviewServiceMock, realtimeSvc *mocks.RealtimeUsageServiceMock) *controllers.RealtimeController {
	return controllers.NewRealtimeController(interviewSvc, realtimeSvc)
}

// ---- Token ----

func TestRealtimeController_Token_MissingFields(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"user_id": 0, "interview_id": 0})
	req := httptest.NewRequest(http.MethodPost, "/api/realtime/token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, newRealtimeController(nil, nil).Token, newCtx(req, rec), http.StatusBadRequest)
}

func TestRealtimeController_Token_Forbidden(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"user_id": 1, "interview_id": 2})
	req := httptest.NewRequest(http.MethodPost, "/api/realtime/token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	svc := &mocks.InterviewServiceMock{}
	svc.On("CreateRealtimeToken", tmock.Anything, uint(1), uint(2)).Return("", errors.New("forbidden"))
	assertStatus(t, newRealtimeController(svc, nil).Token, newCtx(req, rec), http.StatusForbidden)
}

func TestRealtimeController_Token_Success(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"user_id": 1, "interview_id": 2})
	req := httptest.NewRequest(http.MethodPost, "/api/realtime/token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	svc := &mocks.InterviewServiceMock{}
	svc.On("CreateRealtimeToken", tmock.Anything, uint(1), uint(2)).Return("ephemeral-secret-token", nil)
	assertStatus(t, newRealtimeController(svc, nil).Token, newCtx(req, rec), http.StatusOK)

	var resp map[string]string
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "ephemeral-secret-token", resp["client_secret"])
	svc.AssertExpectations(t)
}

func TestRealtimeController_Token_TooManyRequests(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"user_id": 1, "interview_id": 2})
	req := httptest.NewRequest(http.MethodPost, "/api/realtime/token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	svc := &mocks.InterviewServiceMock{}
	svc.On("CreateRealtimeToken", tmock.Anything, uint(1), uint(2)).Return("", errors.New("realtime capacity exceeded: max 10 concurrent sessions"))
	assertStatus(t, newRealtimeController(svc, nil).Token, newCtx(req, rec), http.StatusTooManyRequests)
}

// ---- SessionInfo ----

func TestRealtimeController_SessionInfo_WithService(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/realtime/session-info", nil)
	rec := httptest.NewRecorder()

	rtSvc := &mocks.RealtimeUsageServiceMock{}
	rtSvc.On("SessionDurationMinutes").Return(15)
	assertStatus(t, newRealtimeController(nil, rtSvc).SessionInfo, newCtx(req, rec), http.StatusOK)

	var resp map[string]int
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 15, resp["session_minutes"])
	rtSvc.AssertExpectations(t)
}

func TestRealtimeController_SessionInfo_NilService(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/realtime/session-info", nil)
	rec := httptest.NewRecorder()
	// nil インターフェースを直接渡すとデフォルト10分が返る
	ctrl := controllers.NewRealtimeController(nil, nil)
	assertStatus(t, ctrl.SessionInfo, newCtx(req, rec), http.StatusOK)

	var resp map[string]int
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 10, resp["session_minutes"])
}

// ========== IntegratedProfileController ==========

func newIntegratedProfileController(
	cf *mocks.CrossFeatureServiceMock,
	sc *mocks.InterviewSessionCounterMock,
	rd *mocks.ResumeDocumentFinderMock,
) *controllers.IntegratedProfileController {
	return controllers.NewIntegratedProfileController(cf, sc, rd)
}

func TestIntegratedProfileController_GetProfile_MissingUserID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/user/profile", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newIntegratedProfileController(nil, nil, nil).GetProfile, newCtx(req, rec), http.StatusBadRequest)
}

func TestIntegratedProfileController_GetProfile_MissingSessionID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/user/profile?user_id=1", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newIntegratedProfileController(nil, nil, nil).GetProfile, newCtx(req, rec), http.StatusBadRequest)
}

func TestIntegratedProfileController_GetProfile_InvalidUserID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/user/profile?user_id=abc&session_id=sess-1", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newIntegratedProfileController(nil, nil, nil).GetProfile, newCtx(req, rec), http.StatusBadRequest)
}

func TestIntegratedProfileController_GetProfile_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/user/profile?user_id=1&session_id=sess-abc", nil)
	rec := httptest.NewRecorder()

	cf := &mocks.CrossFeatureServiceMock{}
	sc := &mocks.InterviewSessionCounterMock{}
	rd := &mocks.ResumeDocumentFinderMock{}

	sc.On("CountByUser", uint(1)).Return(int64(3), nil)
	rd.On("FindDocumentsByUserID", uint(1)).Return([]models.ResumeDocument{{Status: "reviewed"}}, nil)
	cf.On("BuildIntegratedProfile", uint(1), "sess-abc", 3, true).Return(&services.UserIntegratedProfile{UserID: 1}, nil)

	assertStatus(t, newIntegratedProfileController(cf, sc, rd).GetProfile, newCtx(req, rec), http.StatusOK)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	cf.AssertExpectations(t)
	sc.AssertExpectations(t)
	rd.AssertExpectations(t)
}

func TestIntegratedProfileController_GetProfile_ServiceError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/user/profile?user_id=1&session_id=sess-abc", nil)
	rec := httptest.NewRecorder()

	cf := &mocks.CrossFeatureServiceMock{}
	sc := &mocks.InterviewSessionCounterMock{}
	rd := &mocks.ResumeDocumentFinderMock{}

	sc.On("CountByUser", uint(1)).Return(int64(0), nil)
	rd.On("FindDocumentsByUserID", uint(1)).Return([]models.ResumeDocument{}, nil)
	cf.On("BuildIntegratedProfile", uint(1), "sess-abc", 0, false).Return(nil, errors.New("service failure"))

	assertStatus(t, newIntegratedProfileController(cf, sc, rd).GetProfile, newCtx(req, rec), http.StatusInternalServerError)
}
