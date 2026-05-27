package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/middleware"
	"net/http"

	"github.com/labstack/echo/v4"
)

// SetupAuthRoutes 認証関連のルーティング設定
func SetupAuthRoutes(api *echo.Group, authController *controllers.AuthController, oauthController *controllers.OAuthController, userSecret string) {
	auth := api.Group("/auth")

	// 認証不要エンドポイント
	auth.POST("/request-registration", authController.RequestRegistration)
	auth.POST("/verify-registration", authController.VerifyRegistration)
	auth.POST("/register", authController.Register)
	// レート制限ミドルウェアはnet/httpベースのため echo.WrapMiddleware で変換して適用
	auth.POST("/login", authController.Login, echoLoginRateLimit())
	auth.POST("/guest", authController.CreateGuest)
	auth.GET("/verify-email", authController.VerifyEmail)
	auth.POST("/forgot-password", authController.RequestPasswordReset, echoPasswordResetRateLimit())
	auth.POST("/reset-password", authController.ResetPassword)
	auth.GET("/google", oauthController.GoogleLogin)
	auth.GET("/google/callback", oauthController.GoogleCallback)
	auth.GET("/github", oauthController.GitHubLogin)
	auth.GET("/github/callback", oauthController.GitHubCallback)

	// 認証必須エンドポイント（X-User-Token JWTヘッダーが必要）
	authProtected := api.Group("/auth", EchoUserAuth(userSecret))
	authProtected.GET("/user", authController.GetUser)
	authProtected.PUT("/profile", authController.UpdateProfile)
	authProtected.DELETE("/account", authController.DeleteAccount)
}

// echoLoginRateLimit はログインエンドポイント用のEchoネイティブレート制限ミドルウェア
func echoLoginRateLimit() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := middleware.GetClientIP(c.Request())
			if !middleware.LoginRateLimiter.Allow(ip) {
				return echo.NewHTTPError(http.StatusTooManyRequests, "Too Many Requests: お試し回数の上限に達しました。しばらく待ってから再試行してください。")
			}
			return next(c)
		}
	}
}

// echoPasswordResetRateLimit はパスワードリセットエンドポイント用のEchoネイティブレート制限ミドルウェア
func echoPasswordResetRateLimit() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := middleware.GetClientIP(c.Request())
			if !middleware.PasswordResetRateLimiter.Allow(ip) {
				return echo.NewHTTPError(http.StatusTooManyRequests, "Too Many Requests: リクエスト上限に達しました。しばらく待ってから再試行してください。")
			}
			return next(c)
		}
	}
}
