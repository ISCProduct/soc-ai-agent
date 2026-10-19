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
	// 職員ロール（role=staff）の判定を返す。ログイン後にチャットを出さず
	// 教員指導画面（/admin）へ送るのに使う。
	resp.IsStaff = user.HasStaffRole()
	// 管理者に加えて職員にも管理者トークンを発行する（管理APIへ到達するため）。
	if user.CanAccessAdminArea() && adminSecret != "" {
		resp.Token = middleware.GenerateAdminToken(user.ID, user.Email, adminSecret)
	}
	if userSecret != "" {
		resp.UserToken = middleware.GenerateUserToken(user.ID, user.Email, userSecret)
		if includeRefresh {
			resp.RefreshToken = s.issueRefreshToken(user.ID)
		}
	}
}
