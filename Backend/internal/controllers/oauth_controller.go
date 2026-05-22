package controllers

import (
	"Backend/internal/config"
	"Backend/internal/middleware"
	"Backend/internal/services"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

type OAuthController struct {
	oauthService *services.OAuthService
}

func NewOAuthController(oauthService *services.OAuthService) *OAuthController {
	return &OAuthController{oauthService: oauthService}
}

// GoogleLogin Google OAuth認証開始
// GET /api/auth/google
func (c *OAuthController) GoogleLogin(ctx echo.Context) error {
	// state を生成して HttpOnly Cookie に保存（CSRF 対策 #324）
	state, err := middleware.GenerateOAuthState(ctx.Response().Writer)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Internal Server Error")
	}

	url := c.oauthService.GetGoogleAuthURL(state)

	return ctx.JSON(http.StatusOK, map[string]string{
		"auth_url": url,
	})
}

// GoogleCallback Google OAuth認証コールバック
// GET /api/auth/google/callback
func (c *OAuthController) GoogleCallback(ctx echo.Context) error {
	// state 検証（CSRF 対策 #324）
	if !middleware.VerifyOAuthState(ctx.Response().Writer, ctx.Request()) {
		log.Printf("[OAuth] Google callback: invalid or missing state from %s", ctx.Request().RemoteAddr)
		return ctx.Redirect(http.StatusTemporaryRedirect, config.AppURL()+"?error=auth_failed")
	}

	code := ctx.QueryParam("code")
	if code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Authorization code not found")
	}

	resp, err := c.oauthService.HandleGoogleCallback(ctx.Request().Context(), code)
	if err != nil {
		// エラー詳細はサーバーログにのみ記録し、クライアントには汎用コードを返す（#329）
		log.Printf("[OAuth] Google callback error: %v", err)
		return ctx.Redirect(http.StatusTemporaryRedirect, config.AppURL()+"?error=auth_failed")
	}

	userData, _ := json.Marshal(resp)
	redirectURL := config.AppURL() + "/auth/callback?provider=google&user=" + base64.URLEncoding.EncodeToString(userData)
	return ctx.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// GitHubLogin GitHub OAuth認証開始
// GET /api/auth/github
func (c *OAuthController) GitHubLogin(ctx echo.Context) error {
	// state を生成して HttpOnly Cookie に保存（CSRF 対策 #324）
	state, err := middleware.GenerateOAuthState(ctx.Response().Writer)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Internal Server Error")
	}

	url := c.oauthService.GetGitHubAuthURL(state)

	return ctx.JSON(http.StatusOK, map[string]string{
		"auth_url": url,
	})
}

// GitHubCallback GitHub OAuth認証コールバック
// GET /api/auth/github/callback
func (c *OAuthController) GitHubCallback(ctx echo.Context) error {
	// state 検証（CSRF 対策 #324）
	if !middleware.VerifyOAuthState(ctx.Response().Writer, ctx.Request()) {
		log.Printf("[OAuth] GitHub callback: invalid or missing state from %s", ctx.Request().RemoteAddr)
		return ctx.Redirect(http.StatusTemporaryRedirect, config.AppURL()+"?error=auth_failed")
	}

	code := ctx.QueryParam("code")
	if code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Authorization code not found")
	}

	resp, err := c.oauthService.HandleGitHubCallback(ctx.Request().Context(), code)
	if err != nil {
		// エラー詳細はサーバーログにのみ記録し、クライアントには汎用コードを返す（#329）
		log.Printf("[OAuth] GitHub callback error: %v", err)
		return ctx.Redirect(http.StatusTemporaryRedirect, config.AppURL()+"?error=auth_failed")
	}

	userData, _ := json.Marshal(resp)
	redirectURL := config.AppURL() + "/auth/callback?provider=github&user=" + base64.URLEncoding.EncodeToString(userData)
	return ctx.Redirect(http.StatusTemporaryRedirect, redirectURL)
}
