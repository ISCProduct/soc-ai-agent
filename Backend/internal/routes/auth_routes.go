package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/middleware"
	"Backend/internal/repositories"
	"net/http"
)

// SetupAuthRoutes 認証関連のルーティング設定
func SetupAuthRoutes(authController *controllers.AuthController, oauthController *controllers.OAuthController, userRepo *repositories.UserRepository, userSecret string) {
	userAuth := func(f http.HandlerFunc) http.HandlerFunc {
		return middleware.UserAuthFunc(userRepo, userSecret, f)
	}

	// 認証エンドポイント（認証不要）
	http.HandleFunc("/api/auth/request-registration", authController.RequestRegistration)
	http.HandleFunc("/api/auth/verify-registration", authController.VerifyRegistration)
	http.HandleFunc("/api/auth/register", authController.Register)
	http.HandleFunc("/api/auth/login", middleware.LoginRateLimit(authController.Login))
	http.HandleFunc("/api/auth/guest", authController.CreateGuest)
	http.HandleFunc("/api/auth/verify-email", authController.VerifyEmail)
	http.HandleFunc("/api/auth/forgot-password", middleware.PasswordResetRateLimit(authController.RequestPasswordReset))
	http.HandleFunc("/api/auth/reset-password", authController.ResetPassword)

	// 認証必須エンドポイント（X-User-ID + X-User-Token ヘッダーが必要）
	http.HandleFunc("/api/auth/user", userAuth(authController.GetUser))
	http.HandleFunc("/api/auth/profile", userAuth(authController.UpdateProfile))
	http.HandleFunc("/api/auth/account", userAuth(authController.DeleteAccount))

	// OAuth エンドポイント
	http.HandleFunc("/api/auth/google", oauthController.GoogleLogin)
	http.HandleFunc("/api/auth/google/callback", oauthController.GoogleCallback)
	http.HandleFunc("/api/auth/github", oauthController.GitHubLogin)
	http.HandleFunc("/api/auth/github/callback", oauthController.GitHubCallback)
}
