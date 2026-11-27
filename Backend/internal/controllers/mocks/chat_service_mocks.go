package mocks

import (
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

// GetChatHistoryForUser は user_id でスコープした履歴取得（#1156）。
func (m *ChatServiceMock) GetChatHistoryForUser(sessionID string, userID uint) ([]models.ChatMessage, error) {
	args := m.Called(sessionID, userID)
	if v := args.Get(0); v != nil {
		return v.([]models.ChatMessage), args.Error(1)
	}
	return nil, args.Error(1)
}

// SessionHasOtherUserMessages は session_id に自分以外のメッセージがあるか（#1156）。
func (m *ChatServiceMock) SessionHasOtherUserMessages(sessionID string, userID uint) (bool, error) {
	args := m.Called(sessionID, userID)
	return args.Bool(0), args.Error(1)
}

// ClaimSessionOwnership は session_id の初回所有者を原子的に確定する（#963）。
func (m *ChatServiceMock) ClaimSessionOwnership(sessionID string, userID uint) error {
	args := m.Called(sessionID, userID)
	return args.Error(0)
}

func (m *ChatServiceMock) GetUserScores(userID uint, sessionID string) ([]models.UserWeightScore, error) {
	args := m.Called(userID, sessionID)
	if v := args.Get(0); v != nil {
		return v.([]models.UserWeightScore), args.Error(1)
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

func (m *MatchingServiceMock) GetTopMatches(ctx context.Context, userID uint, sessionID string, limit int) ([]*models.UserCompanyMatch, error) {
	args := m.Called(ctx, userID, sessionID, limit)
	if v := args.Get(0); v != nil {
		return v.([]*models.UserCompanyMatch), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MatchingServiceMock) ToggleFavorite(matchID uint, userID uint) error {
	args := m.Called(matchID, userID)
	return args.Error(0)
}

func (m *MatchingServiceMock) ListFavoriteCompanies(_ context.Context, userID uint, offset, limit int) ([]*models.UserCompanyMatch, int, []uint, error) {
	args := m.Called(userID, offset, limit)
	var page []*models.UserCompanyMatch
	if v := args.Get(0); v != nil {
		page = v.([]*models.UserCompanyMatch)
	}
	total := 0
	if v := args.Get(1); v != nil {
		total = v.(int)
	}
	var companyIDs []uint
	if v := args.Get(2); v != nil {
		companyIDs = v.([]uint)
	}
	return page, total, companyIDs, args.Error(3)
}

func (m *MatchingServiceMock) UnfavoriteCompany(_ context.Context, userID uint, companyID uint) error {
	args := m.Called(userID, companyID)
	return args.Error(0)
}

func (m *MatchingServiceMock) GetDiagnostics(userID uint, sessionID string) (*matching.MatchingDiagnostics, error) {
	args := m.Called(userID, sessionID)
	if v := args.Get(0); v != nil {
		return v.(*matching.MatchingDiagnostics), args.Error(1)
	}
	return nil, args.Error(1)
}

// ReasonScores は合流しない（そのまま返す）。合流の検証は matching パッケージのテストで行う。
func (m *MatchingServiceMock) ReasonScores(_ uint, scores []models.UserWeightScore) []models.UserWeightScore {
	return scores
}

func (m *MatchingServiceMock) GetAxisBreakdown(matchID, userID uint) (*matching.AxisBreakdown, error) {
	args := m.Called(matchID, userID)
	if v := args.Get(0); v != nil {
		return v.(*matching.AxisBreakdown), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MatchingServiceMock) GetMyAxisScores(userID uint, sessionID string) ([]matching.MyAxisScore, error) {
	args := m.Called(userID, sessionID)
	if v := args.Get(0); v != nil {
		return v.([]matching.MyAxisScore), args.Error(1)
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

func (m *EmailServiceMock) SendAnalysisReport(user *models.User, summary *analysis.AnalysisSummary, companies []email.EmailReportCompany, sessionID string) error {
	args := m.Called(user, summary, companies, sessionID)
	return args.Error(0)
}

func (m *EmailServiceMock) SendSystemAlertEmail(recipients []string, subject, body string) error {
	args := m.Called(recipients, subject, body)
	return args.Error(0)
}
