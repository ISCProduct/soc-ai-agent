package services_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/models"
	openaiPkg "Backend/internal/openai"
	"Backend/internal/services"

	openai "github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
)

// newOpenAITestServer は簡易な OpenAI テストサーバーを起動して指定の応答を返します。
func newOpenAITestServerForAnalysis(t *testing.T, responseBody string) (*httptest.Server, *openaiPkg.Client) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := openai.ChatCompletionResponse{
			ID:    "test-id",
			Model: "gpt-4o",
			Choices: []openai.ChatCompletionChoice{
				{
					Message: openai.ChatCompletionMessage{
						Role:    "assistant",
						Content: responseBody,
					},
					FinishReason: "stop",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv, openaiPkg.NewWithBaseURL(srv.URL, "gpt-4o")
}

// mock implementations
type mockChatMessageRepo struct{}

func (m *mockChatMessageRepo) Create(msg *models.ChatMessage) error { return nil }
func (m *mockChatMessageRepo) FindBySessionID(sessionID string) ([]models.ChatMessage, error) {
	return nil, nil
}
func (m *mockChatMessageRepo) FindByUserID(userID uint) ([]models.ChatMessage, error) {
	return nil, nil
}
func (m *mockChatMessageRepo) FindRecentBySessionID(sessionID string, limit int) ([]models.ChatMessage, error) {
	return []models.ChatMessage{
		{Role: "user", Content: "私はバックエンド開発が好きで、チームでの協調を重視します。"},
		{Role: "user", Content: "新しい技術に挑戦したいです。"},
	}, nil
}
func (m *mockChatMessageRepo) GetUsedQuestionIDs(sessionID string) ([]uint, error) { return nil, nil }
func (m *mockChatMessageRepo) GetUserSessions(userID uint) ([]models.ChatSession, error) {
	return nil, nil
}

type mockConversationContextRepo struct {
	saved string
}

func (m *mockConversationContextRepo) GetBySessionID(sessionID string) (*models.ConversationContext, error) {
	return &models.ConversationContext{SessionID: sessionID}, nil
}
func (m *mockConversationContextRepo) GetOrCreate(userID uint, sessionID string) (*models.ConversationContext, error) {
	return &models.ConversationContext{UserID: userID, SessionID: sessionID}, nil
}
func (m *mockConversationContextRepo) SetJobCategoryID(userID uint, sessionID string, jobCategoryID uint) error {
	return nil
}
func (m *mockConversationContextRepo) GetJobCategoryID(sessionID string) (uint, error) { return 0, nil }
func (m *mockConversationContextRepo) SetSessionSummary(userID uint, sessionID string, summary string) error {
	m.saved = summary
	return nil
}

type mockUserWeightScoreRepo struct{}

func (m *mockUserWeightScoreRepo) SetScore(userID uint, sessionID, category string, absoluteScore int) error {
	return nil
}
func (m *mockUserWeightScoreRepo) AddScore(userID uint, sessionID, category string, delta int) error {
	return nil
}
func (m *mockUserWeightScoreRepo) FindByUserAndSession(userID uint, sessionID string) ([]entity.UserWeightScore, error) {
	return []entity.UserWeightScore{
		{UserID: userID, SessionID: sessionID, WeightCategory: "技術志向", Score: 80},
		{UserID: userID, SessionID: sessionID, WeightCategory: "チームワーク志向", Score: 75},
	}, nil
}
func (m *mockUserWeightScoreRepo) FindTopCategories(userID uint, sessionID string, limit int) ([]entity.UserWeightScore, error) {
	scores, err := m.FindByUserAndSession(userID, sessionID)
	if err != nil || len(scores) <= limit {
		return scores, err
	}
	return scores[:limit], nil
}
func (m *mockUserWeightScoreRepo) FindByUserSessionAndCategory(userID uint, sessionID, category string) (*entity.UserWeightScore, error) {
	return &entity.UserWeightScore{UserID: userID, SessionID: sessionID, WeightCategory: category, Score: 80}, nil
}
func (m *mockUserWeightScoreRepo) CountByUserAndSession(userID uint, sessionID string) (int64, error) {
	return 2, nil
}

func TestBuildAnalysisSummary_WithLLM(t *testing.T) {
	// モック OpenAI サーバー: 構造化JSONを返す
	responseBody := `{"strengths": ["技術志向"], "concerns": ["面接準備不足"], "recommended_working_style": "チームで協調しつつ個人で裁量を持つ"}`
	srv, aiClient := newOpenAITestServerForAnalysis(t, responseBody)
	_ = srv

	chatRepo := &mockChatMessageRepo{}
	convRepo := &mockConversationContextRepo{}
	scoreRepo := &mockUserWeightScoreRepo{}

	svc := services.NewAnalysisScoringService(scoreRepo, chatRepo, nil, convRepo, nil, nil, nil, aiClient, nil)

	summary, err := svc.BuildAnalysisSummary(context.Background(), 1, "session-123")
	assert.NoError(t, err)
	assert.NotNil(t, summary)
	assert.Equal(t, responseBody, summary.LLMRawSummary)
	assert.NotNil(t, summary.LLMStructured)
	assert.Contains(t, summary.LLMStructured.Strengths, "技術志向")
	// 永続化が呼ばれていること
	assert.Equal(t, responseBody, convRepo.saved)
}
