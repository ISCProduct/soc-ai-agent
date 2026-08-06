package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

func SetupCollectiveInsightRoutes(api *echo.Group, controller *controllers.CollectiveInsightController, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver) {
	ci := api.Group("/collective-insights", EchoUserAuth(userSecret, access, orgs))
	ci.GET("/recommendations", controller.GetRecommendations)
	ci.GET("/top-companies", controller.GetTopPassRateCompanies)
	ci.PUT("/consent", controller.UpdateConsent)
	ci.POST("/actions", controller.RecordAction)
}
