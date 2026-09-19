package routes

import (
	"Backend/internal/controllers"

	"github.com/labstack/echo/v4"
)

func SetupESRoutes(api *echo.Group, esRewriteController *controllers.ESRewriteController, esReviewController *controllers.ESReviewController) {
	// 未ログイン利用が仕様のため認証は付けられない。コスト濫用はレート制限で止める（#1154）
	es := api.Group("/es", echoGuestAIRateLimit())
	es.POST("/rewrite", esRewriteController.Rewrite)
	es.POST("/review", esReviewController.Review)
}
