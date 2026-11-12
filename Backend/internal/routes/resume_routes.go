package routes

import (
	resumecontrollers "Backend/internal/controllers/resume"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

func SetupResumeRoutes(api *echo.Group, resumeController *resumecontrollers.ResumeController, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver, budget AIBudgetGuards) {
	resume := api.Group("/resume", EchoUserAuth(userSecret, access, orgs))
	resume.POST("/upload", resumeController.Upload)
	resume.POST("/review", resumeController.Review, budget.Immediate)
	resume.POST("/review/stream", resumeController.ReviewStream, budget.Immediate)
	resume.GET("/status", resumeController.Status)
	resume.GET("/annotated", resumeController.Annotated)
}
