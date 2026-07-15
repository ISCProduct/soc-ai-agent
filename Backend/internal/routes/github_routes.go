package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/services"

	"github.com/labstack/echo/v4"
)

// SetupGitHubRoutes GitHub連携関連のルーティング設定
func SetupGitHubRoutes(api *echo.Group, githubController *controllers.GitHubController, userSecret string, access services.UserAccessGuard) {
	github := api.Group("/github", EchoUserAuth(userSecret, access))
	github.GET("/profile", githubController.GetProfile)
	github.POST("/sync", githubController.Sync)
	github.POST("/sync/wait", githubController.SyncAndWait)
	github.GET("/skills", githubController.GetSkills)
	github.GET("/repo/summaries", githubController.ListRepoSummaries)
	github.POST("/repo/summarize", githubController.SummarizeRepo)
}
