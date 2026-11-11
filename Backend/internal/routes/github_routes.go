package routes

import (
	githubcontrollers "Backend/internal/controllers/github"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

// SetupGitHubRoutes GitHub連携関連のルーティング設定
func SetupGitHubRoutes(api *echo.Group, githubController *githubcontrollers.GitHubController, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver, budget AIBudgetGuards) {
	github := api.Group("/github", EchoUserAuth(userSecret, access, orgs))
	github.GET("/profile", githubController.GetProfile)
	github.POST("/sync", githubController.Sync)
	github.POST("/sync/wait", githubController.SyncAndWait)
	github.GET("/skills", githubController.GetSkills)
	github.GET("/repo/summaries", githubController.ListRepoSummaries)
	// 保存済みの要約を返すときは AI を呼ばないので、判定を AI の直前まで遅らせる。
	github.POST("/repo/summarize", githubController.SummarizeRepo, budget.Deferred)
}
