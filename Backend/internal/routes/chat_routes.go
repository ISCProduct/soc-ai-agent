package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/middleware"
	"net/http"
)

// SetupChatRoutes チャット関連のルーティング設定
func SetupChatRoutes(chatController *controllers.ChatController, questionController *controllers.QuestionController, userSecret string) {
	userAuth := func(f http.HandlerFunc) http.HandlerFunc {
		return middleware.UserAuthFunc(userSecret, f)
	}

	// チャットエンドポイント（認証必須）
	http.HandleFunc("/api/chat", userAuth(chatController.Chat))
	http.HandleFunc("/api/chat/history", userAuth(chatController.GetHistory))
	http.HandleFunc("/api/chat/scores", userAuth(chatController.GetScores))
	http.HandleFunc("/api/chat/recommendations", userAuth(chatController.GetRecommendations))
	http.HandleFunc("/api/chat/analysis", userAuth(chatController.GetAnalysisSummary))
	http.HandleFunc("/api/chat/sessions", userAuth(chatController.GetSessions))
	http.HandleFunc("/api/chat/send-report", userAuth(chatController.SendReport))
	http.HandleFunc("/api/chat/favorite", userAuth(chatController.ToggleFavorite))

	// 質問管理エンドポイント（認証必須）
	http.HandleFunc("/api/questions/generate", userAuth(questionController.GenerateQuestions))
	http.HandleFunc("/api/questions/create", userAuth(questionController.CreateQuestion))
	http.HandleFunc("/api/questions/list", userAuth(questionController.GetQuestionsByCategory))
}
