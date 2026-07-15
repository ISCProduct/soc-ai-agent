package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/services"

	"github.com/labstack/echo/v4"
)

// SetupChatRoutes チャット関連のルーティング設定
func SetupChatRoutes(api *echo.Group, chatController *controllers.ChatController, questionController *controllers.QuestionController, userSecret string, access services.UserAccessGuard) {
	// チャットエンドポイント（認証必須）
	chat := api.Group("/chat", EchoUserAuth(userSecret, access))
	chat.POST("", chatController.Chat)
	chat.GET("/history", chatController.GetHistory)
	chat.GET("/scores", chatController.GetScores)
	chat.GET("/recommendations", chatController.GetRecommendations)
	chat.GET("/analysis", chatController.GetAnalysisSummary)
	chat.GET("/sessions", chatController.GetSessions)
	chat.POST("/send-report", chatController.SendReport)
	chat.POST("/favorite", chatController.ToggleFavorite)

	// 質問管理エンドポイント（認証必須）
	questions := api.Group("/questions", EchoUserAuth(userSecret, access))
	questions.POST("/generate", questionController.GenerateQuestions)
	questions.POST("/create", questionController.CreateQuestion)
	questions.GET("/list", questionController.GetQuestionsByCategory)
}
