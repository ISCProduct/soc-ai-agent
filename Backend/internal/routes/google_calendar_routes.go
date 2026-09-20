package routes

import (
	schedulecontrollers "Backend/internal/controllers/schedule"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

func SetupGoogleCalendarRoutes(api *echo.Group, calendarController *schedulecontrollers.GoogleCalendarController, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver) {
	// 認証不要（OAuth コールバック）
	api.GET("/google-calendar/callback", calendarController.ConnectCallback)

	// 認証必須エンドポイント
	cal := api.Group("/google-calendar", EchoUserAuth(userSecret, access, orgs))
	cal.GET("/connect", calendarController.ConnectStart)
	cal.GET("/status", calendarController.Status)
	cal.DELETE("/disconnect", calendarController.Disconnect)
}
