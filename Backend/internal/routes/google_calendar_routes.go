package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/services"

	"github.com/labstack/echo/v4"
)

func SetupGoogleCalendarRoutes(api *echo.Group, calendarController *controllers.GoogleCalendarController, userSecret string, access services.UserAccessGuard) {
	// 認証不要（OAuth コールバック）
	api.GET("/google-calendar/callback", calendarController.ConnectCallback)

	// 認証必須エンドポイント
	cal := api.Group("/google-calendar", EchoUserAuth(userSecret, access))
	cal.GET("/connect", calendarController.ConnectStart)
	cal.GET("/status", calendarController.Status)
	cal.DELETE("/disconnect", calendarController.Disconnect)
}
