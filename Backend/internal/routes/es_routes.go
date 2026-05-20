package routes

import (
	"Backend/internal/controllers"

	"github.com/labstack/echo/v4"
)

func SetupESRoutes(api *echo.Group, esRewriteController *controllers.ESRewriteController, esReviewController *controllers.ESReviewController) {
	es := api.Group("/es")
	es.Any("/rewrite", wrap(esRewriteController.Rewrite))
	es.Any("/review", wrap(esReviewController.Review))
}
