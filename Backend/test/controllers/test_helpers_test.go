package controllers_test

import (
	"context"
	"net/http"

	"Backend/internal/middleware"
)

// withUserID はリクエストコンテキストにuserIDを設定するヘルパー
func withUserID(r *http.Request, userID uint) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey, userID)
	return r.WithContext(ctx)
}
