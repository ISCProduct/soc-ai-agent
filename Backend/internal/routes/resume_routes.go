package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/services"

	"github.com/labstack/echo/v4"
)

func SetupResumeRoutes(api *echo.Group, resumeController *controllers.ResumeController, userSecret string, access services.UserAccessGuard) {
	resume := api.Group("/resume", EchoUserAuth(userSecret, access))
	resume.POST("/upload", resumeController.Upload)
	resume.POST("/review", resumeController.Review)
	resume.POST("/review/stream", resumeController.ReviewStream)
	resume.GET("/annotated", resumeController.Annotated)
}
