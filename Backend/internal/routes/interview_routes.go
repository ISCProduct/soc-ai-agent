package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/middleware"
	"Backend/internal/repositories"
	"net/http"
)

// SetupInterviewRoutes 面接関連のルーティング設定
func SetupInterviewRoutes(interviewController *controllers.InterviewController, realtimeController *controllers.RealtimeController, userRepo *repositories.UserRepository, userSecret string) {
	userAuth := func(f http.HandlerFunc) http.HandlerFunc {
		return middleware.UserAuthFunc(userRepo, userSecret, f)
	}

	// /api/interviews/trend はワイルドカード /api/interviews/ より先に登録する必要がある
	http.HandleFunc("/api/interviews/trend", userAuth(interviewController.GetTrend))
	http.HandleFunc("/api/interviews", userAuth(interviewController.ListOrCreate))
	http.HandleFunc("/api/interviews/", userAuth(interviewController.Route))
	http.HandleFunc("/api/realtime/token", userAuth(realtimeController.Token))
	http.HandleFunc("/api/realtime/session-info", userAuth(realtimeController.SessionInfo))
}
