package middleware

import (
	"context"
	"net/http"
)

type contextKey string

const UserIDContextKey contextKey = "userID"

// GenerateUserToken はJWTユーザートークンを生成する
func GenerateUserToken(userID uint, email, secret string) string {
	token, err := GenerateJWT(userID, email, secret)
	if err != nil {
		return ""
	}
	return token
}

// VerifyUserToken はJWTトークンを検証する（後方互換性のため残存）
func VerifyUserToken(token string, userID uint, _ string, secret string) bool {
	parsedID, _, err := ParseJWT(token, secret)
	if err != nil {
		return false
	}
	return parsedID == userID
}

// UserAuthFunc は X-User-Token ヘッダーのJWTを検証し、ユーザーIDをコンテキストに保存するミドルウェア
// userSecret が未設定の場合はフェイルクローズ（503）として動作する
func UserAuthFunc(userSecret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if userSecret == "" {
			http.Error(w, "Service Unavailable: authentication not configured", http.StatusServiceUnavailable)
			return
		}

		token := r.Header.Get("X-User-Token")
		if token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userID, _, err := ParseJWT(token, userSecret)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDContextKey, userID)
		next(w, r.WithContext(ctx))
	}
}
