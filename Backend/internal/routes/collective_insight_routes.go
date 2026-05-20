package routes

import (
	"Backend/internal/controllers"

	"github.com/labstack/echo/v4"
)

func SetupCollectiveInsightRoutes(api *echo.Group, controller *controllers.CollectiveInsightController, userSecret string) {
	ci := api.Group("/collective-insights", EchoUserAuth(userSecret))
	ci.Any("/recommendations", wrap(controller.Route))
	ci.Any("/top-companies", wrap(controller.Route))
	ci.Any("/consent", wrap(controller.Route))
	ci.Any("/actions", wrap(controller.Route))
}
