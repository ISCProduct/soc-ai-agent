package routes

import (
	assessmentcontrollers "Backend/internal/controllers/assessment"
	"Backend/internal/repositories"
	"Backend/internal/services/auth"
	"Backend/internal/services/school"

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

// SetupAssessmentSchoolRoutes は学校独自テンプレートと先生向け受検状況（#1675）。
// 担当校スコープ（EchoAdminSchoolScope）の下に置く。職員（staff）も担当校内で使える。
// SetupAdminRoutes の外に置くので、admin_routes の分類表には載らない。
func SetupAssessmentSchoolRoutes(api *echo.Group, c *assessmentcontrollers.SchoolController, userRepo *repositories.UserRepository, schools *school.SchoolService, adminSecret string) {
	g := api.Group("/admin/assessment-templates", EchoAdminAuth(userRepo, adminSecret), EchoAdminSchoolScope(schools))
	g.GET("", c.List)
	g.POST("", c.Create)
	g.GET("/:id/students", c.Students)
}
