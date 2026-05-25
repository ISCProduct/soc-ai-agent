package routes

import (
	"Backend/internal/controllers"

	"github.com/labstack/echo/v4"
)

func SetupCollectiveInsightRoutes(api *echo.Group, controller *controllers.CollectiveInsightController, userSecret string) {
	ci := api.Group("/collective-insights", EchoUserAuth(userSecret))
	ci.GET("/recommendations", controller.GetRecommendations)
	ci.GET("/top-companies", controller.GetTopPassRateCompanies)
	ci.PUT("/consent", controller.UpdateConsent)
	ci.POST("/actions", controller.RecordAction)
}
