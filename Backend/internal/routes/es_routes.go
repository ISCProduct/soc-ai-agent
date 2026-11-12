package routes

import (
	escontrollers "Backend/internal/controllers/es"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

func SetupESRoutes(api *echo.Group, esRewriteController *escontrollers.ESRewriteController, esReviewController *escontrollers.ESReviewController,
	userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver, budget AIBudgetGuards) {
	// 未ログイン利用が仕様のため認証は必須にできない。コスト濫用はレート制限で止める（#1154）。
	// ただし添削結果を履歴として残すには実行者が要るので、トークンがあれば載せる（#1531）。
	// 未ログインのときは従来どおり素通りし、保存だけ行わない。予算もログイン中の利用者にだけ適用する。
	es := api.Group("/es", echoGuestAIRateLimit(), EchoOptionalUserAuth(userSecret, access, orgs))
	es.POST("/rewrite", esRewriteController.Rewrite, budget.Immediate)
	es.POST("/review", esReviewController.Review, budget.Immediate)
}
