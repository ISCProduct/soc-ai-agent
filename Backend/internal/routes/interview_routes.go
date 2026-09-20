package routes

import (
	interviewcontrollers "Backend/internal/controllers/interview"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// SetupInterviewRoutes 面接関連のルーティング設定
func SetupInterviewRoutes(api *echo.Group, interviewController *interviewcontrollers.InterviewController, realtimeController *interviewcontrollers.RealtimeController, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver) {
	interviews := api.Group("/interviews", EchoUserAuth(userSecret, access, orgs))
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
	// 動画だけはグローバルの 32M 制限から除外してあるので、ここで上限を置く
	// （maxVideoSize=500MB + multipart のオーバーヘッド分）。
	interviews.POST("/:id/upload-video", interviewController.UploadVideo, echomw.BodyLimit("512M"))
	interviews.GET("/:id/phrase-suggestions", interviewController.GetPhraseSuggestions)
	interviews.POST("/:id/turn", interviewController.Turn)
	interviews.POST("/:id/start-turn", interviewController.StartTurn)

	realtime := api.Group("/realtime", EchoUserAuth(userSecret, access, orgs))
	realtime.POST("/token", realtimeController.Token)
	realtime.GET("/session-info", realtimeController.SessionInfo)

	hr := api.Group("/hr", EchoUserAuth(userSecret, access, orgs))
	hr.GET("/interviews", interviewController.HRList)
}
