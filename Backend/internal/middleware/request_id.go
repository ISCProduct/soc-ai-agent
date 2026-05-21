package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const RequestIDHeader = "X-Request-ID"
const requestIDContextKey contextKey = "requestID"

func generateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// RequestIDMiddleware はリクエストごとに一意の ID を付与し、
// レスポンスヘッダー X-Request-ID とコンテキストにセットする。
// クライアントが X-Request-ID を送信した場合はその値を優先する。
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get(RequestIDHeader)
		if rid == "" {
			rid = generateRequestID()
		}
		w.Header().Set(RequestIDHeader, rid)
		ctx := context.WithValue(r.Context(), requestIDContextKey, rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID はコンテキストからリクエスト ID を取得する。
func GetRequestID(ctx context.Context) string {
	v, _ := ctx.Value(requestIDContextKey).(string)
	return v
}
