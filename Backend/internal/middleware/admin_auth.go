package middleware

import (
	"Backend/internal/repositories"
	"net/http"
)

// AdminAuth X-Admin-Email と X-Admin-Token ヘッダーでユーザーを検証し is_admin が true であることを確認する
func AdminAuth(userRepo *repositories.UserRepository, adminSecret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !verifyAdminRequest(w, r, userRepo, adminSecret) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AdminAuthFunc AdminAuth の http.HandlerFunc バージョン
func AdminAuthFunc(userRepo *repositories.UserRepository, adminSecret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !verifyAdminRequest(w, r, userRepo, adminSecret) {
			return
		}
		next(w, r)
	}
}

// verifyAdminRequest はリクエストの管理者認証を検証する共通ロジック
func verifyAdminRequest(w http.ResponseWriter, r *http.Request, userRepo *repositories.UserRepository, adminSecret string) bool {
	email := r.Header.Get("X-Admin-Email")
	if email == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	user, err := userRepo.GetUserByEmail(email)
	if err != nil || user == nil || !user.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return false
	}
	// ADMIN_SECRET 未設定の場合はフェイルクローズ（セキュリティ設定漏れを防ぐ）
	if adminSecret == "" {
		http.Error(w, "Service Unavailable: admin authentication not configured", http.StatusServiceUnavailable)
		return false
	}
	token := r.Header.Get("X-Admin-Token")
	if token == "" || !VerifyAdminToken(token, user.ID, user.Email, adminSecret) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return false
	}
	return true
}
