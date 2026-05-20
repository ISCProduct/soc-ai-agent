package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/repositories"

	"github.com/labstack/echo/v4"
)

func SetupResumeRoutes(api *echo.Group, resumeController *controllers.ResumeController, userRepo *repositories.UserRepository, userSecret string) {
	resume := api.Group("/resume", EchoUserAuth(userRepo, userSecret))
	resume.Any("/upload", wrap(resumeController.Upload))
	resume.Any("/review", wrap(resumeController.Review))
	resume.Any("/review/stream", wrap(resumeController.ReviewStream))
	resume.Any("/annotated", wrap(resumeController.Annotated))
}
