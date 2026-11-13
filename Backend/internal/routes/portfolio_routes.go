package routes

import (
	portfoliocontrollers "Backend/internal/controllers/portfolio"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

// SetupPortfolioRoutes は自己分析ポートフォリオの API を登録する（#2030）。
// EchoUserAuth で本人を決め、user_id はクエリから受け取らない。
func SetupPortfolioRoutes(api *echo.Group, c *portfoliocontrollers.Controller, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver) {
	g := api.Group("/portfolio", EchoUserAuth(userSecret, access, orgs))
	g.GET("", c.Get)
}
