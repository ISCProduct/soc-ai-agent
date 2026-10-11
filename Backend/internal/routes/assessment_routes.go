package routes

import (
	assessmentcontrollers "Backend/internal/controllers/assessment"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

// SetupAssessmentRoutes は学生の適性検査（#1678）。全て本人認証の下に置く。
func SetupAssessmentRoutes(api *echo.Group, c *assessmentcontrollers.Controller, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver) {
	g := api.Group("/assessments", EchoUserAuth(userSecret, access, orgs))
	g.GET("", c.List)
	g.POST("/:templateId/sessions", c.Start)
	g.GET("/sessions/:sessionId", c.Get)
	g.PUT("/sessions/:sessionId/answers/:questionId", c.SaveAnswer)
	g.POST("/sessions/:sessionId/submit", c.Submit)
	g.GET("/sessions/:sessionId/result", c.Result)
}
