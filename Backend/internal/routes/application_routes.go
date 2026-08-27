package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

// SetupApplicationRoutes 応募・選考ステータス管理のルーティング設定
func SetupApplicationRoutes(api *echo.Group, appController *controllers.ApplicationController, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver) {
	applications := api.Group("/applications", EchoUserAuth(userSecret, access, orgs))
	// POST /api/applications       → 応募登録
	// GET  /api/applications       → 応募一覧取得
	applications.POST("", appController.Apply)
	applications.GET("", appController.List)
	// GET  /api/applications/correlation → 相関分析データ
	applications.GET("/correlation", appController.GetCorrelation)
	// PUT  /api/applications/:id  → ステータス更新（互換維持。isAdminは常にfalse）
	applications.PUT("/:id", appController.UpdateStatus)
	// POST /api/applications/:id/withdraw → 選考辞退
	// POST /api/applications/:id/accept   → 内定承諾
	applications.POST("/:id/withdraw", appController.Withdraw)
	applications.POST("/:id/accept", appController.Accept)

	// 企業オーナー向け（#1083）。company_id 必須、所有権がなければ 403。
	hr := api.Group("/hr", EchoUserAuth(userSecret, access, orgs))
	hr.GET("/applications", appController.HRList)
	hr.PATCH("/applications/:id/status", appController.HRUpdateStatus)
}
