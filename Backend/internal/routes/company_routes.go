package routes

import (
	"Backend/internal/controllers"

	"github.com/labstack/echo/v4"
)

// SetupCompanyRoutes 企業関連のルーティング設定
func SetupCompanyRoutes(api *echo.Group, relationController *controllers.CompanyRelationController) {
	companies := api.Group("/companies")
	companies.GET("", relationController.GetCompanies)
	companies.GET("/relations", relationController.GetAllCompanyRelations)
	companies.GET("/market-info", relationController.GetAllMarketInfo)
	companies.GET("/web-search", relationController.WebSearchCompanies)
	// 固定パスを :id より先に登録してEchoが優先解決する（順序は不問だがドキュメント目的で明示）
	companies.GET("/:id", relationController.GetCompanyByID)
	companies.GET("/:id/job-positions", relationController.GetCompanyJobPositions)
	companies.GET("/:id/relations", relationController.GetCompanyRelations)
	companies.GET("/:id/market-info", relationController.GetCompanyMarketInfo)
}
