package routes

import (
	"Backend/internal/controllers"

	"github.com/labstack/echo/v4"
)

// SetupApplicationRoutes 応募・選考ステータス管理のルーティング設定
func SetupApplicationRoutes(api *echo.Group, appController *controllers.ApplicationController) {
	applications := api.Group("/applications")
	// POST /api/applications       → 応募登録
	// GET  /api/applications       → 応募一覧取得
	applications.POST("", appController.Apply)
	applications.GET("", appController.List)
	// GET  /api/applications/correlation → 相関分析データ
	applications.GET("/correlation", appController.GetCorrelation)
	// PUT  /api/applications/:id  → ステータス更新
	applications.PUT("/:id", appController.UpdateStatus)
}
