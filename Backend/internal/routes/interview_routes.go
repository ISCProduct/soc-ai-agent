package routes

import (
	interviewcontrollers "Backend/internal/controllers/interview"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// SetupInterviewRoutes 面接関連のルーティング設定
func SetupInterviewRoutes(api *echo.Group, interviewController *interviewcontrollers.InterviewController, realtimeController *interviewcontrollers.RealtimeController, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver, budget AIBudgetGuards) {
	// 予算は面接を始める時点で判定する。開始後の発話や終了を止めると体験が壊れるので、
	// turn / utterances / finish には付けない。
	interviews := api.Group("/interviews", EchoUserAuth(userSecret, access, orgs))
	// /trend は /:id より先にEchoのルーターが解決するため先に登録する
	interviews.GET("/trend", interviewController.GetTrend)
	interviews.GET("", interviewController.List)
	interviews.POST("", interviewController.Create)
	interviews.GET("/:id", interviewController.Get)
	interviews.POST("/:id/start", interviewController.Start, budget.Immediate)
	interviews.POST("/:id/finish", interviewController.Finish)
	interviews.POST("/:id/utterances", interviewController.AddUtterance)
	interviews.GET("/:id/report", interviewController.GetReport)
	// 生成ジョブが失われたレポートをユーザー操作で作り直すための回復経路(#1476)
	interviews.POST("/:id/report/regenerate", interviewController.RegenerateReport, budget.Immediate)
	interviews.POST("/:id/send-report", interviewController.SendReport)
	// 動画だけはグローバルの 32M 制限から除外してあるので、ここで上限を置く
	// （maxVideoSize=500MB + multipart のオーバーヘッド分）。
	interviews.POST("/:id/upload-video", interviewController.UploadVideo, echomw.BodyLimit("512M"))
	interviews.GET("/:id/phrase-suggestions", interviewController.GetPhraseSuggestions, budget.Immediate)
	interviews.POST("/:id/turn", interviewController.Turn)
	interviews.POST("/:id/start-turn", interviewController.StartTurn)

	realtime := api.Group("/realtime", EchoUserAuth(userSecret, access, orgs))
	realtime.POST("/token", realtimeController.Token, budget.Immediate)
	realtime.GET("/session-info", realtimeController.SessionInfo)

	hr := api.Group("/hr", EchoUserAuth(userSecret, access, orgs))
	hr.GET("/interviews", interviewController.HRList)
}
