package routes

import (
	"Backend/internal/controllers"

	"github.com/labstack/echo/v4"
)

func SetupUserRoutes(api *echo.Group, profileController *controllers.IntegratedProfileController) {
	user := api.Group("/user")
	user.Any("/profile", wrap(profileController.GetProfile))
}
