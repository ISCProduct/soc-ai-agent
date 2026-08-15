package mocks

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"Backend/internal/services/analysis"
	"Backend/internal/services/chat"
	"Backend/internal/services/email"
	"Backend/internal/services/matching"
	"context"

	"github.com/stretchr/testify/mock"
)

// ChatServiceMock ChatServiceのモック実装
type ChatServiceMock struct {
	mock.Mock
}

func (m *ChatServiceMock) ProcessChat(ctx context.Context, req chat.ChatRequest) (*chat.ChatResponse, error) {
	args := m.Called(ctx, req)
	if v := args.Get(0); v != nil {
		return v.(*chat.ChatResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ChatServiceMock) GetChatHistory(sessionID string) ([]models.ChatMessage, error) {
	args := m.Called(sessionID)
	if v := args.Get(0); v != nil {
		return v.([]models.ChatMessage), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ChatServiceMock) GetUserScores(userID uint, sessionID string) ([]entity.UserWeightScore, error) {
	args := m.Called(userID, sessionID)
	if v := args.Get(0); v != nil {
		return v.([]entity.UserWeightScore), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ChatServiceMock) GetUserChatSessions(userID uint) ([]models.ChatSession, error) {
	args := m.Called(userID)
	if v := args.Get(0); v != nil {
		return v.([]models.ChatSession), args.Error(1)
	}
	return nil, args.Error(1)
}

// MatchingServiceMock MatchingServiceのモック実装
type MatchingServiceMock struct {
	mock.Mock
}

func (m *MatchingServiceMock) CalculateMatching(ctx context.Context, userID uint, sessionID string) error {
	args := m.Called(ctx, userID, sessionID)
	return args.Error(0)
}

func (m *MatchingServiceMock) GetTopMatches(ctx context.Context, userID uint, sessionID string, limit int) ([]*entity.UserCompanyMatch, error) {
	args := m.Called(ctx, userID, sessionID, limit)
	if v := args.Get(0); v != nil {
		return v.([]*entity.UserCompanyMatch), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MatchingServiceMock) ToggleFavorite(matchID uint, userID uint) error {
	args := m.Called(matchID, userID)
	return args.Error(0)
}

func (m *MatchingServiceMock) GetDiagnostics(userID uint, sessionID string) (*matching.MatchingDiagnostics, error) {
	args := m.Called(userID, sessionID)
	if v := args.Get(0); v != nil {
		return v.(*matching.MatchingDiagnostics), args.Error(1)
	}
	return nil, args.Error(1)
}

// AnalysisScoringServiceMock AnalysisScoringServiceのモック実装
type AnalysisScoringServiceMock struct {
	mock.Mock
}

func (m *AnalysisScoringServiceMock) BuildAnalysisSummary(ctx context.Context, userID uint, sessionID string) (*analysis.AnalysisSummary, error) {
	args := m.Called(ctx, userID, sessionID)
	if v := args.Get(0); v != nil {
		return v.(*analysis.AnalysisSummary), args.Error(1)
	}
	return nil, args.Error(1)
}

// EmailServiceMock EmailServiceのモック実装
type EmailServiceMock struct {
	mock.Mock
}

func (m *EmailServiceMock) SendAnalysisReport(user *entity.User, summary *analysis.AnalysisSummary, companies []email.EmailReportCompany, sessionID string) error {
	args := m.Called(user, summary, companies, sessionID)
	return args.Error(0)
}

func (m *EmailServiceMock) SendSystemAlertEmail(recipients []string, subject, body string) error {
	args := m.Called(recipients, subject, body)
	return args.Error(0)
}
