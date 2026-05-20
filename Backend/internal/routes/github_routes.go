package routes

import (
	"Backend/internal/controllers"

	"github.com/labstack/echo/v4"
)

// SetupGitHubRoutes GitHub連携関連のルーティング設定
func SetupGitHubRoutes(api *echo.Group, githubController *controllers.GitHubController, userSecret string) {
	github := api.Group("/github", EchoUserAuth(userSecret))
	github.Any("/profile", wrap(githubController.GetProfile))
	github.Any("/sync", wrap(githubController.Sync))
	github.Any("/sync/wait", wrap(githubController.SyncAndWait))
	github.Any("/skills", wrap(githubController.GetSkills))
	github.Any("/repo/summaries", wrap(githubController.ListRepoSummaries))
	github.Any("/repo/summarize", wrap(githubController.SummarizeRepo))
}
