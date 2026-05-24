package controllers

import (
	"Backend/internal/services"
	ifaces "Backend/internal/services/interfaces"
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

// GitHubController GitHub連携APIのコントローラー
type GitHubController struct {
	githubService     ifaces.GitHubService
	skillScoreService ifaces.SkillScoreService
}

func NewGitHubController(githubService ifaces.GitHubService, skillScoreService ifaces.SkillScoreService) *GitHubController {
	return &GitHubController{
		githubService:     githubService,
		skillScoreService: skillScoreService,
	}
}

// GetProfile GitHubプロフィール・リポジトリ・言語統計を取得する
// GET /api/github/profile
func (c *GitHubController) GetProfile(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	profile, err := c.githubService.GetProfile(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get github profile")
	}
	if profile == nil {
		return echo.NewHTTPError(http.StatusNotFound, "github profile not found")
	}

	repos, err := c.githubService.GetRepositories(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get repositories")
	}

	langStats, err := c.githubService.GetLanguageStats(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get language stats")
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"profile":        profile,
		"repositories":   repos,
		"language_stats": langStats,
	})
}

// Sync GitHubデータの非同期同期をトリガーする
// POST /api/github/sync
func (c *GitHubController) Sync(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	profile, err := c.githubService.GetProfile(userID)
	if err != nil || profile == nil {
		return echo.NewHTTPError(http.StatusNotFound, "github profile not found: please connect your GitHub account")
	}

	force := ctx.QueryParam("force") == "true"
	c.githubService.TriggerAsyncSync(userID, force)

	return ctx.JSON(http.StatusOK, map[string]string{
		"status": "sync started",
	})
}

// SyncAndWait GitHubデータを同期してから結果を返す（同期的）
// POST /api/github/sync/wait
func (c *GitHubController) SyncAndWait(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	if err := c.githubService.SyncUserData(context.Background(), userID, true); err != nil {
		var scopeErr *services.InsufficientScopesError
		if errors.As(err, &scopeErr) {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]string{
		"status": "sync completed",
	})
}

// GetSkills ユーザーのカテゴリ別スキルスコアを取得する
// GET /api/github/skills
func (c *GitHubController) GetSkills(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	scores, err := c.skillScoreService.GetScores(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get skill scores")
	}

	return ctx.JSON(http.StatusOK, scores)
}

// ListRepoSummaries ユーザーのリポジトリAI要約一覧を取得する
// GET /api/github/repo/summaries
func (c *GitHubController) ListRepoSummaries(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	summaries, err := c.githubService.ListRepoSummaries(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get repo summaries")
	}

	return ctx.JSON(http.StatusOK, summaries)
}

// SummarizeRepo リポジトリのAI要約を生成・キャッシュする
// POST /api/github/repo/summarize
// Body: { "full_name": "owner/repo", "force_refresh": false }
func (c *GitHubController) SummarizeRepo(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	var body struct {
		FullName     string `json:"full_name"`
		ForceRefresh bool   `json:"force_refresh"`
	}
	if err := ctx.Bind(&body); err != nil || body.FullName == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "full_name is required")
	}

	summary, err := c.githubService.SummarizeRepo(ctx.Request().Context(), userID, body.FullName, body.ForceRefresh)
	if err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, summary)
}
