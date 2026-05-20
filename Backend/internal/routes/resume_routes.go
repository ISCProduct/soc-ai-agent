package routes

import (
	"Backend/internal/controllers"

	"github.com/labstack/echo/v4"
)

func SetupResumeRoutes(api *echo.Group, resumeController *controllers.ResumeController, userSecret string) {
	resume := api.Group("/resume", EchoUserAuth(userSecret))
	resume.Any("/upload", wrap(resumeController.Upload))
	resume.Any("/review", wrap(resumeController.Review))
	resume.Any("/review/stream", wrap(resumeController.ReviewStream))
	resume.Any("/annotated", wrap(resumeController.Annotated))
}
