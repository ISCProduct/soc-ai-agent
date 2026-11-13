package routes

import (
	graduateoutcomecontrollers "Backend/internal/controllers/graduateoutcome"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

// SetupGraduateOutcomeRoutes は学生向けの卒業生の進路実績の API を登録する（#2032）。
// 学生は EchoUserAuth から決め、その学生の学校の実績だけを返す。
func SetupGraduateOutcomeRoutes(api *echo.Group, c *graduateoutcomecontrollers.Controller, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver) {
	g := api.Group("/graduate-outcomes", EchoUserAuth(userSecret, access, orgs))
	g.GET("", c.List)
	g.GET("/companies/:id", c.Company)
}
