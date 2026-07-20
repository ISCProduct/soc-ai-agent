package routes

import (
	"Backend/internal/middleware"
	"Backend/internal/repositories"
	"Backend/internal/services"
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

// OrganizationIDResolver は認証済みユーザーの組織IDを解決する。
type OrganizationIDResolver interface {
	ResolveOrganizationID(userID uint) (uint, error)
}

// EchoUserAuth はX-User-Token JWTを検証するEcho nativeミドルウェアを返す。
// userSecret が未設定の場合はフェイルクローズ（503）として動作する。
// access が非 nil の場合、退会済みユーザーを遮断する。
// orgs が非 nil の場合、organization_id をリクエストコンテキストへ載せる。
func EchoUserAuth(userSecret string, access services.UserAccessGuard, orgs ...OrganizationIDResolver) echo.MiddlewareFunc {
	var resolver OrganizationIDResolver
	if len(orgs) > 0 {
		resolver = orgs[0]
	}
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
			if access != nil {
				if err := access.EnsureActiveUser(userID); err != nil {
					if errors.Is(err, services.ErrAccountWithdrawn) {
						return echo.NewHTTPError(http.StatusForbidden, "account has been withdrawn")
					}
					return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
				}
			}
			ctx := context.WithValue(c.Request().Context(), middleware.UserIDContextKey, userID)
			if resolver != nil {
				orgID, err := resolver.ResolveOrganizationID(userID)
				if err != nil {
					if errors.Is(err, services.ErrOrganizationDisabled) {
						return echo.NewHTTPError(http.StatusForbidden, "organization is disabled")
					}
					return echo.NewHTTPError(http.StatusInternalServerError, "failed to resolve organization")
				}
				if orgID == 0 {
					return echo.NewHTTPError(http.StatusForbidden, "organization not found")
				}
				ctx = context.WithValue(ctx, middleware.OrganizationIDContextKey, orgID)
			}
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
