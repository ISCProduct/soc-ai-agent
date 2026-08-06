package interfaces

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"Backend/internal/services"
	"Backend/internal/services/analysis"
	"Backend/internal/services/email"
	"Backend/internal/services/matching"
	"context"
)

// ChatService チャットサービスのインターフェース
type ChatService interface {
	ProcessChat(ctx context.Context, req services.ChatRequest) (*services.ChatResponse, error)
	GetChatHistory(sessionID string) ([]models.ChatMessage, error)
	GetUserScores(userID uint, sessionID string) ([]entity.UserWeightScore, error)
	GetUserChatSessions(userID uint) ([]models.ChatSession, error)
}

// MatchingService マッチングサービスのインターフェース
type MatchingService interface {
	CalculateMatching(ctx context.Context, userID uint, sessionID string) error
	GetTopMatches(ctx context.Context, userID uint, sessionID string, limit int) ([]*entity.UserCompanyMatch, error)
	ToggleFavorite(matchID uint, userID uint) error
	GetDiagnostics(userID uint, sessionID string) (*matching.MatchingDiagnostics, error)
}

// AnalysisScoringService 分析・スコアリングサービスのインターフェース
type AnalysisScoringService interface {
	BuildAnalysisSummary(ctx context.Context, userID uint, sessionID string) (*analysis.AnalysisSummary, error)
}

// EmailService メールサービスのインターフェース
type EmailService interface {
	SendAnalysisReport(user *entity.User, summary *analysis.AnalysisSummary, companies []email.EmailReportCompany, sessionID string) error
	SendSystemAlertEmail(recipients []string, subject, body string) error
}
