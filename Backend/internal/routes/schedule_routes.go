package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

// SetupScheduleRoutes はスケジュールAPIを登録する。他機能(chat/resume等)と同様に
// EchoUserAuthでJWT認証を必須にする(#983: 以前はuser_idクエリパラメータのみで
// 認証を経由せず任意ユーザーの予定を読み書きできた)。
func SetupScheduleRoutes(api *echo.Group, scheduleController *controllers.ScheduleController, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver) {
	schedule := api.Group("/schedule", EchoUserAuth(userSecret, access, orgs))
	schedule.GET("/export/ics", scheduleController.ExportICS)
	schedule.GET("", scheduleController.List)
	schedule.POST("", scheduleController.Create)
	schedule.GET("/:id", scheduleController.Get)
	schedule.PUT("/:id", scheduleController.Update)
	schedule.DELETE("/:id", scheduleController.Delete)
}
