package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/middleware"
	"net/http"
)

func SetupResumeRoutes(resumeController *controllers.ResumeController, userSecret string) {
	userAuth := func(f http.HandlerFunc) http.HandlerFunc {
		return middleware.UserAuthFunc(userSecret, f)
	}

	http.HandleFunc("/api/resume/upload", userAuth(resumeController.Upload))
	http.HandleFunc("/api/resume/review", userAuth(resumeController.Review))
	http.HandleFunc("/api/resume/review/stream", userAuth(resumeController.ReviewStream))
	http.HandleFunc("/api/resume/annotated", userAuth(resumeController.Annotated))
}
