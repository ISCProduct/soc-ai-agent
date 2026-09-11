package auth

import (
	"Backend/domain/entity"
	"Backend/internal/middleware"
	"os"
)

// attachAuthTokens はログイン済みユーザー向けの HMAC トークンを resp に付与する。
// includeRefresh が true のときのみ refresh_token を発行する（GetUser では不要）。
func (s *AuthService) attachAuthTokens(resp *AuthResponse, user *entity.User, includeRefresh bool) {
	adminSecret := os.Getenv("ADMIN_SECRET")
	userSecret := os.Getenv("USER_SECRET")
	if user.IsAdmin && adminSecret != "" {
		resp.Token = middleware.GenerateAdminToken(user.ID, user.Email, adminSecret)
	}
	if userSecret != "" {
		resp.UserToken = middleware.GenerateUserToken(user.ID, user.Email, userSecret)
		if includeRefresh {
			resp.RefreshToken = s.issueRefreshToken(user.ID)
		}
	}
}
