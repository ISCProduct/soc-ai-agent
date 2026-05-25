package routes

import (
	"Backend/internal/controllers"

	"github.com/labstack/echo/v4"
)

func SetupResumeRoutes(api *echo.Group, resumeController *controllers.ResumeController, userSecret string) {
	resume := api.Group("/resume", EchoUserAuth(userSecret))
	resume.POST("/upload", resumeController.Upload)
	resume.POST("/review", resumeController.Review)
	resume.GET("/review/stream", resumeController.ReviewStream)
	resume.GET("/annotated", resumeController.Annotated)
}
