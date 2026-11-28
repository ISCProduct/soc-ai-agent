package interfaces

import (
	"Backend/internal/models"
	"Backend/internal/services/analysis"
	"Backend/internal/services/chat"
	"Backend/internal/services/email"
	"Backend/internal/services/matching"
	"context"
)

// ChatService チャットサービスのインターフェース
type ChatService interface {
	ProcessChat(ctx context.Context, req chat.ChatRequest) (*chat.ChatResponse, error)
	// GetChatHistoryForUser / SessionHasOtherUserMessages はクエリを user_id でスコープした
	// 所有者判定に使う（#1156）
	GetChatHistoryForUser(sessionID string, userID uint) ([]models.ChatMessage, error)
	SessionHasOtherUserMessages(sessionID string, userID uint) (bool, error)
	ClaimSessionOwnership(sessionID string, userID uint) error
	GetUserScores(userID uint, sessionID string) ([]models.UserWeightScore, error)
	GetUserChatSessions(userID uint) ([]models.ChatSession, error)
}

// MatchingService マッチングサービスのインターフェース
type MatchingService interface {
	CalculateMatching(ctx context.Context, userID uint, sessionID string) error
	GetTopMatches(ctx context.Context, userID uint, sessionID string, limit int) ([]*models.UserCompanyMatch, error)
	ToggleFavorite(matchID uint, userID uint) error
	ListFavoriteCompanies(ctx context.Context, userID uint, offset, limit int) ([]*models.UserCompanyMatch, int, []uint, error)
	UnfavoriteCompany(ctx context.Context, userID uint, companyID uint) error
	GetDiagnostics(userID uint, sessionID string) (*matching.MatchingDiagnostics, error)
	ReasonScores(userID uint, scores []models.UserWeightScore) []models.UserWeightScore
	// GetAxisBreakdown は本人の推薦1件の、軸別のマッチの内訳（SOCAIAGENT-611）。
	GetAxisBreakdown(matchID, userID uint) (*matching.AxisBreakdown, error)
	// GetMyAxisScores は本人の診断の10軸のスコア（内訳と同じく適性検査を合流させた値。SOCAIAGENT-611）。
	GetMyAxisScores(userID uint, sessionID string) ([]matching.MyAxisScore, error)
}

// AnalysisScoringService 分析・スコアリングサービスのインターフェース
type AnalysisScoringService interface {
	BuildAnalysisSummary(ctx context.Context, userID uint, sessionID string) (*analysis.AnalysisSummary, error)
}

// EmailService メールサービスのインターフェース
type EmailService interface {
	SendAnalysisReport(user *models.User, summary *analysis.AnalysisSummary, companies []email.EmailReportCompany, sessionID string) error
	SendSystemAlertEmail(recipients []string, subject, body string) error
}
