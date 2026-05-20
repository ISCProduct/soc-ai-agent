package controllers

import (
	"Backend/internal/services"
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// GitHubController GitHub連携APIのコントローラー
type GitHubController struct {
	githubService     *services.GitHubService
	skillScoreService *services.SkillScoreService
}

func NewGitHubController(githubService *services.GitHubService, skillScoreService *services.SkillScoreService) *GitHubController {
	return &GitHubController{
		githubService:     githubService,
		skillScoreService: skillScoreService,
	}
}

// GetProfile GitHubプロフィール・リポジトリ・言語統計を取得する
// GET /api/github/profile
func (c *GitHubController) GetProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := authenticatedUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	profile, err := c.githubService.GetProfile(userID)
	if err != nil {
		http.Error(w, "failed to get github profile", http.StatusInternalServerError)
		return
	}
	if profile == nil {
		http.Error(w, "github profile not found", http.StatusNotFound)
		return
	}

	repos, err := c.githubService.GetRepositories(userID)
	if err != nil {
		http.Error(w, "failed to get repositories", http.StatusInternalServerError)
		return
	}

	langStats, err := c.githubService.GetLanguageStats(userID)
	if err != nil {
		http.Error(w, "failed to get language stats", http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"profile":        profile,
		"repositories":   repos,
		"language_stats": langStats,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Sync GitHubデータの非同期同期をトリガーする
// POST /api/github/sync
func (c *GitHubController) Sync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := authenticatedUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	profile, err := c.githubService.GetProfile(userID)
	if err != nil || profile == nil {
		http.Error(w, "github profile not found: please connect your GitHub account", http.StatusNotFound)
		return
	}

	force := r.URL.Query().Get("force") == "true"
	c.githubService.TriggerAsyncSync(userID, force)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "sync started",
	})
}

// SyncAndWait GitHubデータを同期してから結果を返す（同期的）
// POST /api/github/sync/wait
func (c *GitHubController) SyncAndWait(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := authenticatedUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := c.githubService.SyncUserData(context.Background(), userID, true); err != nil {
		var scopeErr *services.InsufficientScopesError
		if errors.As(err, &scopeErr) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		writeInternalServerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "sync completed",
	})
}

// GetSkills ユーザーのカテゴリ別スキルスコアを取得する
// GET /api/github/skills
func (c *GitHubController) GetSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := authenticatedUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	scores, err := c.skillScoreService.GetScores(userID)
	if err != nil {
		http.Error(w, "failed to get skill scores", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scores)
}

// ListRepoSummaries ユーザーのリポジトリAI要約一覧を取得する
// GET /api/github/repo/summaries
func (c *GitHubController) ListRepoSummaries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := authenticatedUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	summaries, err := c.githubService.ListRepoSummaries(userID)
	if err != nil {
		http.Error(w, "failed to get repo summaries", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summaries)
}

// SummarizeRepo リポジトリのAI要約を生成・キャッシュする
// POST /api/github/repo/summarize
// Body: { "full_name": "owner/repo", "force_refresh": false }
func (c *GitHubController) SummarizeRepo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := authenticatedUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		FullName     string `json:"full_name"`
		ForceRefresh bool   `json:"force_refresh"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.FullName == "" {
		http.Error(w, "full_name is required", http.StatusBadRequest)
		return
	}

	summary, err := c.githubService.SummarizeRepo(r.Context(), userID, body.FullName, body.ForceRefresh)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}
