package routes

import (
	"Backend/internal/controllers"

	"github.com/labstack/echo/v4"
)

// SetupInterviewRoutes 面接関連のルーティング設定
func SetupInterviewRoutes(api *echo.Group, interviewController *controllers.InterviewController, realtimeController *controllers.RealtimeController) {
	interviews := api.Group("/interviews")
	// /trend は /*  より先にEchoのルーターが解決するため順序は不問
	interviews.Any("/trend", wrap(interviewController.GetTrend))
	interviews.Any("", wrap(interviewController.ListOrCreate))
	interviews.Any("/*", wrap(interviewController.Route))

	realtime := api.Group("/realtime")
	realtime.Any("/token", wrap(realtimeController.Token))
	realtime.Any("/session-info", wrap(realtimeController.SessionInfo))
}
