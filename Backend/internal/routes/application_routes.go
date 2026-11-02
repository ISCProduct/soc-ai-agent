package routes

import (
	"net/http"

	"Backend/internal/config"
	applicationcontrollers "Backend/internal/controllers/application"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

// SetupApplicationRoutes 応募・選考ステータス管理のルーティング設定
func SetupApplicationRoutes(
	api *echo.Group,
	appController *applicationcontrollers.ApplicationController,
	hrStudentAnalysisController *applicationcontrollers.HRStudentAnalysisController,
	userSecret string,
	access auth.UserAccessGuard,
	orgs OrganizationIDResolver,
) {
	applications := api.Group("/applications", EchoUserAuth(userSecret, access, orgs))
	// POST /api/applications       → 応募登録
	// GET  /api/applications       → 応募一覧取得
	applications.POST("", appController.Apply, requireStudentApply)
	// GET  /api/applications/availability → 応募を受け付けているか（画面のボタン表示用）
	applications.GET("/availability", applyAvailability)
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
	hr.GET("/students/:userID/analysis", hrStudentAnalysisController.GetAnalysis)
}

// requireStudentApply は応募を閉じている間、新規の応募登録を 403 で止める（SOCAIAGENT-403）。
// 画面のボタンを消すだけでは API を直接叩けてしまうため、サーバ側でも拒否する。
func requireStudentApply(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !config.StudentApplyEnabled() {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "応募は現在受け付けていません"})
		}
		return next(c)
	}
}

func applyAvailability(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]bool{"enabled": config.StudentApplyEnabled()})
}
