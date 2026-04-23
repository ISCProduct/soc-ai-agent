package controllers

import (
	"Backend/internal/config"
	"Backend/internal/middleware"
	"Backend/internal/services"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
)

type OAuthController struct {
	oauthService *services.OAuthService
}

func NewOAuthController(oauthService *services.OAuthService) *OAuthController {
	return &OAuthController{oauthService: oauthService}
}

// GoogleLogin Google OAuth認証開始
func (c *OAuthController) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// state を生成して HttpOnly Cookie に保存（CSRF 対策 #324）
	state, err := middleware.GenerateOAuthState(w)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	url := c.oauthService.GetGoogleAuthURL(state)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"auth_url": url,
	})
}

// GoogleCallback Google OAuth認証コールバック
func (c *OAuthController) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// state 検証（CSRF 対策 #324）
	if !middleware.VerifyOAuthState(w, r) {
		log.Printf("[OAuth] Google callback: invalid or missing state from %s", r.RemoteAddr)
		http.Redirect(w, r, config.AppURL()+"?error=auth_failed", http.StatusTemporaryRedirect)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Authorization code not found", http.StatusBadRequest)
		return
	}

	resp, err := c.oauthService.HandleGoogleCallback(r.Context(), code)
	if err != nil {
		// エラー詳細はサーバーログにのみ記録し、クライアントには汎用コードを返す（#329）
		log.Printf("[OAuth] Google callback error: %v", err)
		http.Redirect(w, r, config.AppURL()+"?error=auth_failed", http.StatusTemporaryRedirect)
		return
	}

	userData, _ := json.Marshal(resp)
	redirectURL := config.AppURL() + "/auth/callback?provider=google&user=" + base64.URLEncoding.EncodeToString(userData)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// GitHubLogin GitHub OAuth認証開始
func (c *OAuthController) GitHubLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// state を生成して HttpOnly Cookie に保存（CSRF 対策 #324）
	state, err := middleware.GenerateOAuthState(w)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	url := c.oauthService.GetGitHubAuthURL(state)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"auth_url": url,
	})
}

// GitHubCallback GitHub OAuth認証コールバック
func (c *OAuthController) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// state 検証（CSRF 対策 #324）
	if !middleware.VerifyOAuthState(w, r) {
		log.Printf("[OAuth] GitHub callback: invalid or missing state from %s", r.RemoteAddr)
		http.Redirect(w, r, config.AppURL()+"?error=auth_failed", http.StatusTemporaryRedirect)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Authorization code not found", http.StatusBadRequest)
		return
	}

	resp, err := c.oauthService.HandleGitHubCallback(r.Context(), code)
	if err != nil {
		// エラー詳細はサーバーログにのみ記録し、クライアントには汎用コードを返す（#329）
		log.Printf("[OAuth] GitHub callback error: %v", err)
		http.Redirect(w, r, config.AppURL()+"?error=auth_failed", http.StatusTemporaryRedirect)
		return
	}

	userData, _ := json.Marshal(resp)
	redirectURL := config.AppURL() + "/auth/callback?provider=github&user=" + base64.URLEncoding.EncodeToString(userData)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}
