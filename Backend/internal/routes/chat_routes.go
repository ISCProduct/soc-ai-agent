package routes

import (
	chatcontrollers "Backend/internal/controllers/chat"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

// SetupChatRoutes チャット関連のルーティング設定
func SetupChatRoutes(api *echo.Group, chatController *chatcontrollers.ChatController, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver, budget AIBudgetGuards) {
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
	// 推薦1件の軸別のマッチの内訳（SOCAIAGENT-611）
	chat.GET("/matches/:id/axes", chatController.GetAxisBreakdown)
	// 「あなたの10の傾向」（内訳と同じく適性検査を合流させた値。SOCAIAGENT-611）
	chat.GET("/axis-scores", chatController.GetMyAxisScores)
	chat.GET("/favorites", chatController.GetFavorites)
	chat.POST("/favorites/remove", chatController.RemoveFavoriteCompany)
	// 推薦への「合う／合わない」（SOCAIAGENT-670）
	chat.POST("/match-feedback", chatController.SubmitMatchFeedback)
}
