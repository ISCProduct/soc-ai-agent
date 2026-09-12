package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const RequestIDHeader = "X-Request-ID"
const requestIDContextKey contextKey = "requestID"

// maxRequestIDLen はクライアント指定リクエストIDの上限長。
// ログ肥大とヘッダー汚染を避けるため、これを超える値は破棄して自前生成する。
const maxRequestIDLen = 64

func generateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// isSafeRequestID はクライアント指定のリクエストIDを受け入れてよいか判定する。
// この値はログとRAGへのヘッダーに流れるため、英数字とハイフン・アンダースコアのみ許可する。
func isSafeRequestID(s string) bool {
	if s == "" || len(s) > maxRequestIDLen {
		return false
	}
	for _, c := range []byte(s) {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

// RequestIDMiddleware はリクエストごとに一意の ID を付与し、
// レスポンスヘッダー X-Request-ID とコンテキストにセットする。
// クライアントが X-Request-ID を送信した場合はその値を優先する（形式が安全な場合のみ）。
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get(RequestIDHeader)
		if !isSafeRequestID(rid) {
			rid = generateRequestID()
		}
		w.Header().Set(RequestIDHeader, rid)
		next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), rid)))
	})
}

// WithRequestID はリクエストIDをコンテキストへ載せる。
// リクエストのライフサイクルから切り離したバックグラウンド処理へ
// IDだけを引き継ぐ用途で使う（#1188）。
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey, requestID)
}

// GetRequestID はコンテキストからリクエスト ID を取得する。
func GetRequestID(ctx context.Context) string {
	v, _ := ctx.Value(requestIDContextKey).(string)
	return v
}
