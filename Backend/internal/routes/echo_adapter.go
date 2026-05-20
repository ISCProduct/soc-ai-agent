package routes

import (
	"Backend/internal/middleware"
	"Backend/internal/repositories"
	"net/http"

	"github.com/labstack/echo/v4"
)

// wrap はhttp.HandlerFuncをecho.HandlerFuncに変換する
func wrap(h http.HandlerFunc) echo.HandlerFunc {
	return echo.WrapHandler(h)
}

// EchoUserAuth はX-User-Token JWTを検証するEchoミドルウェアを返す
func EchoUserAuth(userSecret string) echo.MiddlewareFunc {
	return echo.WrapMiddleware(func(next http.Handler) http.Handler {
		return middleware.UserAuthFunc(userSecret, http.HandlerFunc(next.ServeHTTP))
	})
}

// EchoAdminAuth はX-Admin-Email / X-Admin-Tokenヘッダーを検証するEchoミドルウェアを返す
func EchoAdminAuth(userRepo *repositories.UserRepository, adminSecret string) echo.MiddlewareFunc {
	return echo.WrapMiddleware(func(next http.Handler) http.Handler {
		return middleware.AdminAuthFunc(userRepo, adminSecret, http.HandlerFunc(next.ServeHTTP))
	})
}
