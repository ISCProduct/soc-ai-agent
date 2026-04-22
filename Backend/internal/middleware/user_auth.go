package middleware

import (
	"Backend/internal/repositories"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
)

// GenerateUserToken はユーザーID・メールアドレス・シークレットからHMAC-SHA256ユーザートークンを生成する
// 管理者トークンとはペイロードのプレフィックスが異なるため、トークンの流用を防ぐ
func GenerateUserToken(userID uint, email, secret string) string {
	payload := fmt.Sprintf("user:%d:%s", userID, email)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyUserToken はトークンがユーザーID・メールアドレス・シークレットと一致するか検証する
func VerifyUserToken(token string, userID uint, email, secret string) bool {
	expected := GenerateUserToken(userID, email, secret)
	return hmac.Equal([]byte(token), []byte(expected))
}

// UserAuthFunc は X-User-ID と X-User-Token ヘッダーでリクエストを認証するミドルウェア
// userSecret が未設定の場合はフェイルクローズ（503）として動作する
func UserAuthFunc(userRepo *repositories.UserRepository, userSecret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if userSecret == "" {
			http.Error(w, "Service Unavailable: authentication not configured", http.StatusServiceUnavailable)
			return
		}

		userIDStr := r.Header.Get("X-User-ID")
		token := r.Header.Get("X-User-Token")
		if userIDStr == "" || token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := userRepo.GetUserByID(uint(userID))
		if err != nil || user == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if !VerifyUserToken(token, user.ID, user.Email, userSecret) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}
