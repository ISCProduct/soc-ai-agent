package routes

import (
	"Backend/internal/middleware"
	"Backend/internal/repositories"
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
)

// EchoUserAuth はX-User-Token JWTを検証するEcho nativeミドルウェアを返す。
// userSecret が未設定の場合はフェイルクローズ（503）として動作する。
func EchoUserAuth(userSecret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if userSecret == "" {
				return echo.NewHTTPError(http.StatusServiceUnavailable, "Service Unavailable: authentication not configured")
			}
			token := c.Request().Header.Get("X-User-Token")
			if token == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
			}
			userID, _, err := middleware.ParseJWT(token, userSecret)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
			}
			// ユーザーIDをリクエストコンテキストに保存
			ctx := context.WithValue(c.Request().Context(), middleware.UserIDContextKey, userID)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// EchoAdminAuth はX-Admin-Email / X-Admin-Tokenヘッダーを検証するEcho nativeミドルウェアを返す。
func EchoAdminAuth(userRepo *repositories.UserRepository, adminSecret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			email := c.Request().Header.Get("X-Admin-Email")
			if email == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
			}
			user, err := userRepo.GetUserByEmail(email)
			if err != nil || user == nil || !user.IsAdmin {
				return echo.NewHTTPError(http.StatusForbidden, "Forbidden")
			}
			// ADMIN_SECRET 未設定の場合はフェイルクローズ（セキュリティ設定漏れを防ぐ）
			if adminSecret == "" {
				return echo.NewHTTPError(http.StatusServiceUnavailable, "Service Unavailable: admin authentication not configured")
			}
			token := c.Request().Header.Get("X-Admin-Token")
			if token == "" || !middleware.VerifyAdminToken(token, user.ID, user.Email, adminSecret) {
				return echo.NewHTTPError(http.StatusForbidden, "Forbidden")
			}
			return next(c)
		}
	}
}
