package routes

import (
	companysearchcontrollers "Backend/internal/controllers/companysearch"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

// SetupCompanySearchRoutes は学生向けの企業検索の API を登録する（SOCAIAGENT-612）。
// 検索と選択肢は既存の GET /api/companies と同じく認証なし。お気に入りの状態は本人の推薦に付くので EchoUserAuth の下。
func SetupCompanySearchRoutes(api *echo.Group, c *companysearchcontrollers.Controller, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver) {
	api.GET("/companies/search", c.Search)
	api.GET("/companies/search-options", c.Options)
	api.GET("/companies/search/favorites", c.Favorites, EchoUserAuth(userSecret, access, orgs))
}
