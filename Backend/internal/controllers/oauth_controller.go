package controllers

import (
	"Backend/internal/config"
	"Backend/internal/middleware"
	ifaces "Backend/internal/services/interfaces"
	"Backend/internal/services/organization"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
)

type OAuthController struct {
	oauthService  ifaces.OAuthService
	organizations *organization.OrganizationService
}

func NewOAuthController(oauthService ifaces.OAuthService, organizations *organization.OrganizationService) *OAuthController {
	return &OAuthController{oauthService: oauthService, organizations: organizations}
}

// resolveTenantOrgID はテナントslugからOrganizationIDを解決する。
// slugが空、または解決に失敗した場合は0（テナント制約なし）を返す。
func (c *OAuthController) resolveTenantOrgID(slug string) uint {
	if slug == "" || c.organizations == nil {
		return 0
	}
	org, err := c.organizations.ResolveBySlug(slug)
	if err != nil {
		return 0
	}
	return org.ID
}

// tenantRedirectBase はOAuthエラー時のリダイレクト先ベースURLを返す。
// slugが実在する学園を指す場合はその学園サブドメイン、それ以外(空/未登録/不正な値)は
// 従来通りルートドメイン(config.AppURL())にフォールバックする。DB検証済みのorg.Slugを
// 使うため、Cookie由来の生の値をそのままURLに埋め込むことはない(オープンリダイレクト対策)。
func (c *OAuthController) tenantRedirectBase(slug string) string {
	appURL := config.AppURL()
	if slug == "" || c.organizations == nil {
		return appURL
	}
	org, err := c.organizations.ResolveBySlug(slug)
	if err != nil {
		return appURL
	}
	return buildTenantURL(appURL, org.Slug)
}

// buildTenantURL は appURL のホストへ slug をサブドメインとして付与する。
// slugが空、またはappURLがホストを持たない(パース失敗・相対URL等)場合はappURLをそのまま返す。
func buildTenantURL(appURL, slug string) string {
	if slug == "" {
		return appURL
	}
	parsed, err := url.Parse(appURL)
	if err != nil || parsed.Host == "" {
		return appURL
	}
	parsed.Host = slug + "." + parsed.Host
	return parsed.String()
}

// GoogleLogin Google OAuth認証開始
// GET /api/auth/google
func (c *OAuthController) GoogleLogin(ctx echo.Context) error {
	// state を生成して HttpOnly Cookie に保存（CSRF 対策 #324）
	state, err := middleware.GenerateOAuthState(ctx.Response().Writer)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Internal Server Error")
	}
	// 学園サブドメインのテナントslugをCookieに保存し、コールバック時に引き継ぐ
	middleware.SetOAuthTenantSlug(ctx.Response().Writer, ctx.QueryParam("tenant"))

	url := c.oauthService.GetGoogleAuthURL(state)

	// ブラウザを直接 Google OAuth へリダイレクト
	// クッキーがバックエンドと同一オリジンで設定されるため state 検証が正常に機能する
	return ctx.Redirect(http.StatusTemporaryRedirect, url)
}

// GoogleCallback Google OAuth認証コールバック
// GET /api/auth/google/callback
func (c *OAuthController) GoogleCallback(ctx echo.Context) error {
	// state Cookieより先にテナントslug Cookieを消費する(state検証に失敗した場合も
	// エラーリダイレクト先として使うため。両者は独立したCookieで管理されている)。
	tenantSlug := middleware.ConsumeOAuthTenantSlug(ctx.Response().Writer, ctx.Request())
	redirectBase := c.tenantRedirectBase(tenantSlug)

	// state 検証（CSRF 対策 #324）
	if !middleware.VerifyOAuthState(ctx.Response().Writer, ctx.Request()) {
		log.Printf("[OAuth] Google callback: invalid or missing state from %s", ctx.Request().RemoteAddr)
		return ctx.Redirect(http.StatusTemporaryRedirect, redirectBase+"?error=auth_failed")
	}

	code := ctx.QueryParam("code")
	if code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Authorization code not found")
	}

	resp, err := c.oauthService.HandleGoogleCallback(ctx.Request().Context(), code, c.resolveTenantOrgID(tenantSlug))
	if err != nil {
		// エラー詳細はサーバーログにのみ記録し、クライアントには汎用コードを返す（#329）
		log.Printf("[OAuth] Google callback error: %v", err)
		return ctx.Redirect(http.StatusTemporaryRedirect, redirectBase+"?error=auth_failed")
	}

	userData, _ := json.Marshal(resp)
	redirectURL := redirectBase + "/auth/callback?provider=google&user=" + base64.URLEncoding.EncodeToString(userData)
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
	// 学園サブドメインのテナントslugをCookieに保存し、コールバック時に引き継ぐ
	middleware.SetOAuthTenantSlug(ctx.Response().Writer, ctx.QueryParam("tenant"))

	url := c.oauthService.GetGitHubAuthURL(state)

	// Accept ヘッダーで JSON が要求されている場合（後方互換）はJSONを返す
	if ctx.Request().Header.Get("Accept") == "application/json" {
		return ctx.JSON(http.StatusOK, map[string]string{
			"auth_url": url,
			"state":    state,
		})
	}

	// ブラウザ直接アクセスはリダイレクト（クッキーが同一オリジンで正しく設定される）
	return ctx.Redirect(http.StatusTemporaryRedirect, url)
}

// GitHubCallback GitHub OAuth認証コールバック
// GET /api/auth/github/callback
func (c *OAuthController) GitHubCallback(ctx echo.Context) error {
	// state Cookieより先にテナントslug Cookieを消費する(state検証に失敗した場合も
	// エラーリダイレクト先として使うため。両者は独立したCookieで管理されている)。
	tenantSlug := middleware.ConsumeOAuthTenantSlug(ctx.Response().Writer, ctx.Request())
	redirectBase := c.tenantRedirectBase(tenantSlug)

	// state 検証（CSRF 対策 #324）
	if !middleware.VerifyOAuthState(ctx.Response().Writer, ctx.Request()) {
		log.Printf("[OAuth] GitHub callback: invalid or missing state from %s", ctx.Request().RemoteAddr)
		return ctx.Redirect(http.StatusTemporaryRedirect, redirectBase+"?error=auth_failed")
	}

	code := ctx.QueryParam("code")
	if code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Authorization code not found")
	}

	resp, err := c.oauthService.HandleGitHubCallback(ctx.Request().Context(), code, c.resolveTenantOrgID(tenantSlug))
	if err != nil {
		// エラー詳細はサーバーログにのみ記録し、クライアントには汎用コードを返す（#329）
		log.Printf("[OAuth] GitHub callback error: %v", err)
		return ctx.Redirect(http.StatusTemporaryRedirect, redirectBase+"?error=auth_failed")
	}

	userData, _ := json.Marshal(resp)
	redirectURL := redirectBase + "/auth/callback?provider=github&user=" + base64.URLEncoding.EncodeToString(userData)
	return ctx.Redirect(http.StatusTemporaryRedirect, redirectURL)
}
