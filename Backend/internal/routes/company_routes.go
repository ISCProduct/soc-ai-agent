package routes

import (
	"Backend/internal/controllers"

	"github.com/labstack/echo/v4"
)

// SetupCompanyRoutes 企業関連のルーティング設定
func SetupCompanyRoutes(api *echo.Group, relationController *controllers.CompanyRelationController) {
	companies := api.Group("/companies")
	companies.Any("", wrap(relationController.GetCompanies))
	companies.Any("/relations", wrap(relationController.GetAllCompanyRelations))
	companies.Any("/market-info", wrap(relationController.GetAllMarketInfo))
	companies.Any("/web-search", wrap(relationController.WebSearchCompanies))
	// 固定パスを :id より先に登録してEchoが優先解決する（順序は不問だがドキュメント目的で明示）
	companies.Any("/:id", wrap(relationController.GetCompanyByID))
	companies.Any("/:id/job-positions", wrap(relationController.GetCompanyJobPositions))
}
