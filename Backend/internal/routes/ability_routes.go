package routes

import (
	abilitycontrollers "Backend/internal/controllers/ability"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

// SetupAbilityRoutes は学生の能力検査（SOCAIAGENT-433）。全て本人認証の下に置く。
//
// 性格検査（/api/assessments）とパスを分ける。採点方式も結果の意味も違うため、
// 同じパスに相乗りさせると「どちらの検査か」をクエリで分けることになり、
// 取り違えたときに正解や他方の結果が出る経路ができる。
func SetupAbilityRoutes(api *echo.Group, c *abilitycontrollers.Controller, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver) {
	g := api.Group("/ability-assessments", EchoUserAuth(userSecret, access, orgs))
	g.GET("", c.List)
	g.POST("/:templateId/sessions", c.Start)
	g.GET("/sessions/:sessionId", c.Get)
	g.PUT("/sessions/:sessionId/answers/:questionId", c.SaveAnswer)
	g.POST("/sessions/:sessionId/submit", c.Submit)
	g.GET("/sessions/:sessionId/result", c.Result)
}
