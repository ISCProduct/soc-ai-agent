package routes

import (
	"Backend/internal/controllers"

	"github.com/labstack/echo/v4"
)

// SetupInterviewRoutes 面接関連のルーティング設定
func SetupInterviewRoutes(api *echo.Group, interviewController *controllers.InterviewController, realtimeController *controllers.RealtimeController) {
	interviews := api.Group("/interviews")
	// /trend は /:id より先にEchoのルーターが解決するため先に登録する
	interviews.GET("/trend", interviewController.GetTrend)
	interviews.GET("", interviewController.List)
	interviews.POST("", interviewController.Create)
	interviews.GET("/:id", interviewController.Get)
	interviews.POST("/:id/start", interviewController.Start)
	interviews.POST("/:id/finish", interviewController.Finish)
	interviews.POST("/:id/utterances", interviewController.AddUtterance)
	interviews.GET("/:id/report", interviewController.GetReport)
	interviews.POST("/:id/send-report", interviewController.SendReport)
	interviews.POST("/:id/upload-video", interviewController.UploadVideo)
	interviews.GET("/:id/phrase-suggestions", interviewController.GetPhraseSuggestions)
	interviews.POST("/:id/turn", interviewController.Turn)
	interviews.POST("/:id/start-turn", interviewController.StartTurn)

	realtime := api.Group("/realtime")
	realtime.POST("/token", realtimeController.Token)
	realtime.GET("/session-info", realtimeController.SessionInfo)
}
