package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/repositories"

	"github.com/labstack/echo/v4"
)

// SetupChatRoutes チャット関連のルーティング設定
func SetupChatRoutes(api *echo.Group, chatController *controllers.ChatController, questionController *controllers.QuestionController, userRepo *repositories.UserRepository, userSecret string) {
	// チャットエンドポイント（認証必須）
	chat := api.Group("/chat", EchoUserAuth(userRepo, userSecret))
	chat.Any("", wrap(chatController.Chat))
	chat.Any("/history", wrap(chatController.GetHistory))
	chat.Any("/scores", wrap(chatController.GetScores))
	chat.Any("/recommendations", wrap(chatController.GetRecommendations))
	chat.Any("/analysis", wrap(chatController.GetAnalysisSummary))
	chat.Any("/sessions", wrap(chatController.GetSessions))
	chat.Any("/send-report", wrap(chatController.SendReport))
	chat.Any("/favorite", wrap(chatController.ToggleFavorite))

	// 質問管理エンドポイント（認証必須）
	questions := api.Group("/questions", EchoUserAuth(userRepo, userSecret))
	questions.Any("/generate", wrap(questionController.GenerateQuestions))
	questions.Any("/create", wrap(questionController.CreateQuestion))
	questions.Any("/list", wrap(questionController.GetQuestionsByCategory))
}
