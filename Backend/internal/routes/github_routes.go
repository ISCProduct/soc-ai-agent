package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/middleware"
	"net/http"
)

// SetupGitHubRoutes GitHub連携関連のルーティング設定
func SetupGitHubRoutes(githubController *controllers.GitHubController, userSecret string) {
	userAuth := func(f http.HandlerFunc) http.HandlerFunc {
		return middleware.UserAuthFunc(userSecret, f)
	}

	http.HandleFunc("/api/github/profile", userAuth(githubController.GetProfile))
	http.HandleFunc("/api/github/sync", userAuth(githubController.Sync))
	http.HandleFunc("/api/github/sync/wait", userAuth(githubController.SyncAndWait))
	http.HandleFunc("/api/github/skills", userAuth(githubController.GetSkills))
	http.HandleFunc("/api/github/repo/summaries", userAuth(githubController.ListRepoSummaries))
	http.HandleFunc("/api/github/repo/summarize", userAuth(githubController.SummarizeRepo))
}
