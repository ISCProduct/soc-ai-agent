package routes

import (
	chatcontrollers "Backend/internal/controllers/chat"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

// SetupChatRoutes チャット関連のルーティング設定
func SetupChatRoutes(api *echo.Group, chatController *chatcontrollers.ChatController, questionController *chatcontrollers.QuestionController, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver, budget AIBudgetGuards) {
	// チャットエンドポイント（認証必須）
	chat := api.Group("/chat", EchoUserAuth(userSecret, access, orgs))
	chat.POST("", chatController.Chat, budget.Immediate)
	chat.GET("/history", chatController.GetHistory)
	chat.GET("/scores", chatController.GetScores)
	chat.GET("/recommendations", chatController.GetRecommendations)
	chat.GET("/analysis", chatController.GetAnalysisSummary, budget.Immediate)
	chat.GET("/sessions", chatController.GetSessions)
	chat.POST("/send-report", chatController.SendReport, budget.Immediate)
	chat.POST("/favorite", chatController.ToggleFavorite)
	chat.GET("/favorites", chatController.GetFavorites)
	chat.POST("/favorites/remove", chatController.RemoveFavoriteCompany)
	// 推薦への「合う／合わない」（SOCAIAGENT-670）
	chat.POST("/match-feedback", chatController.SubmitMatchFeedback)

	// 質問管理エンドポイント（認証必須）
	questions := api.Group("/questions", EchoUserAuth(userSecret, access, orgs))
	questions.POST("/generate", questionController.GenerateQuestions, budget.Immediate)
	questions.POST("/create", questionController.CreateQuestion)
	questions.GET("/list", questionController.GetQuestionsByCategory)
}
