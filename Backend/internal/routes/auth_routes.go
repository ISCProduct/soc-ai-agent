package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/middleware"

	"github.com/labstack/echo/v4"
)

// SetupAuthRoutes 認証関連のルーティング設定
func SetupAuthRoutes(api *echo.Group, authController *controllers.AuthController, oauthController *controllers.OAuthController, userSecret string) {
	auth := api.Group("/auth")

	// 認証不要エンドポイント
	auth.Any("/request-registration", wrap(authController.RequestRegistration))
	auth.Any("/verify-registration", wrap(authController.VerifyRegistration))
	auth.Any("/register", wrap(authController.Register))
	auth.Any("/login", wrap(middleware.LoginRateLimit(authController.Login)))
	auth.Any("/guest", wrap(authController.CreateGuest))
	auth.Any("/verify-email", wrap(authController.VerifyEmail))
	auth.Any("/forgot-password", wrap(middleware.PasswordResetRateLimit(authController.RequestPasswordReset)))
	auth.Any("/reset-password", wrap(authController.ResetPassword))
	auth.Any("/google", wrap(oauthController.GoogleLogin))
	auth.Any("/google/callback", wrap(oauthController.GoogleCallback))
	auth.Any("/github", wrap(oauthController.GitHubLogin))
	auth.Any("/github/callback", wrap(oauthController.GitHubCallback))

	// 認証必須エンドポイント（X-User-Token JWTヘッダーが必要）
	authProtected := api.Group("/auth", EchoUserAuth(userSecret))
	authProtected.Any("/user", wrap(authController.GetUser))
	authProtected.Any("/profile", wrap(authController.UpdateProfile))
	authProtected.Any("/account", wrap(authController.DeleteAccount))
}
