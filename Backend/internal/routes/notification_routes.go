package routes

import (
	notificationcontrollers "Backend/internal/controllers/notification"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

// SetupNotificationRoutes はアプリ内通知の API を登録する（#2033）。
// EchoUserAuth で本人を決め、user_id はクエリから受け取らない。
func SetupNotificationRoutes(api *echo.Group, c *notificationcontrollers.Controller, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver) {
	g := api.Group("/notifications", EchoUserAuth(userSecret, access, orgs))
	g.GET("", c.List)
	g.GET("/unread-count", c.UnreadCount)
	g.POST("/read-all", c.MarkAllRead)
	g.POST("/:id/read", c.MarkRead)
}
