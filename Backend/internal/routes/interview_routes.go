package routes

import (
	interviewcontrollers "Backend/internal/controllers/interview"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// SetupInterviewRoutes 面接関連のルーティング設定
func SetupInterviewRoutes(api *echo.Group, interviewController *interviewcontrollers.InterviewController, realtimeController *interviewcontrollers.RealtimeController, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver, budget AIBudgetGuards) {
	// 予算は面接を始める時点で判定する。utterances / finish には付けない（開始後の保存や終了を止めると体験が壊れる）。
	// turn / start-turn は判定を context に載せ（Deferred）、サービスが AI を呼ぶ前に読む。
	// ターンでは既定でログのみ（monitor）。超過で止めるのは INTERVIEW_TURN_BUDGET_ENFORCE=true のときだけで、
	// そのときも締めの文で正常終了させる（SOCAIAGENT-696）。
	interviews := api.Group("/interviews", EchoUserAuth(userSecret, access, orgs))
	// /trend と /axis-comparison は /:id より先にEchoのルーターが解決するため先に登録する
	interviews.GET("/trend", interviewController.GetTrend)
	interviews.GET("/axis-comparison", interviewController.CompareAxes)
	interviews.GET("", interviewController.List)
	interviews.POST("", interviewController.Create)
	interviews.GET("/:id", interviewController.Get)
	interviews.POST("/:id/start", interviewController.Start, echoAuthenticatedAIRateLimit(), budget.Immediate)
	interviews.POST("/:id/finish", interviewController.Finish)
	interviews.POST("/:id/utterances", interviewController.AddUtterance)
	interviews.GET("/:id/report", interviewController.GetReport)
	// 生成ジョブが失われたレポートをユーザー操作で作り直すための回復経路(#1476)
	interviews.POST("/:id/report/regenerate", interviewController.RegenerateReport, echoAuthenticatedAIRateLimit(), budget.Immediate)
	interviews.POST("/:id/send-report", interviewController.SendReport)
	// 動画だけはグローバルの 32M 制限から除外してあるので、ここで上限を置く
	// （maxVideoSize=500MB + multipart のオーバーヘッド分）。
	interviews.POST("/:id/upload-video", interviewController.UploadVideo, echomw.BodyLimit("512M"))
	interviews.GET("/:id/phrase-suggestions", interviewController.GetPhraseSuggestions, echoAuthenticatedAIRateLimit(), budget.Immediate)
	interviews.POST("/:id/turn", interviewController.Turn, echoAuthenticatedAIRateLimit(), budget.Deferred)
	interviews.POST("/:id/start-turn", interviewController.StartTurn, echoAuthenticatedAIRateLimit(), budget.Deferred)

	realtime := api.Group("/realtime", EchoUserAuth(userSecret, access, orgs))
	realtime.POST("/token", realtimeController.Token, echoAuthenticatedAIRateLimit(), budget.Immediate)
	realtime.GET("/session-info", realtimeController.SessionInfo)

	hr := api.Group("/hr", EchoUserAuth(userSecret, access, orgs))
	hr.GET("/interviews", interviewController.HRList)
}
