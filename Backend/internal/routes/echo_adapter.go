package routes

import (
	"Backend/internal/middleware"
	"Backend/internal/repositories"
	"Backend/internal/services"
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strconv"

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
				if tenantOrgID, ok := middleware.TenantOrganizationIDFromContext(ctx); ok && tenantOrgID != orgID {
					return echo.NewHTTPError(http.StatusForbidden, "tenant mismatch")
				}
			}
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// EchoTenantResolver は X-Tenant-Slug ヘッダー(学園サブドメインラベル)から組織を解決し、
// リクエストコンテキストへ載せるEcho nativeミドルウェア。
// ヘッダー未指定の場合は何もしない（テナント概念のないアクセスとの後方互換のため）。
func EchoTenantResolver(orgs *services.OrganizationService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			slug := c.Request().Header.Get("X-Tenant-Slug")
			if slug == "" {
				return next(c)
			}
			org, err := orgs.ResolveBySlug(slug)
			if err != nil {
				return echo.NewHTTPError(http.StatusForbidden, "unknown tenant")
			}
			ctx := context.WithValue(c.Request().Context(), middleware.TenantOrganizationIDContextKey, org.ID)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// EchoAdminAuth はX-Admin-Email / X-Admin-Tokenヘッダーを検証するEcho nativeミドルウェアを返す。
// 検証済み管理者のユーザーIDを AdminUserIDContextKey へ格納する(後続のEchoAdminSchoolScope等が利用する)。
// EchoStaticSecretAuth はログインユーザーを介さないサービス間呼び出し（CI等）向けの
// 単純な共有シークレット認証。X-Admin-Secretヘッダーの値をadminSecretと定数時間比較する。
func EchoStaticSecretAuth(adminSecret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if adminSecret == "" {
				return echo.NewHTTPError(http.StatusServiceUnavailable, "Service Unavailable: admin authentication not configured")
			}
			token := c.Request().Header.Get("X-Admin-Secret")
			if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(adminSecret)) != 1 {
				return echo.NewHTTPError(http.StatusForbidden, "Forbidden")
			}
			return next(c)
		}
	}
}

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
			ctx := context.WithValue(c.Request().Context(), middleware.AdminUserIDContextKey, user.ID)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// EchoAdminSchoolScope は管理者(先生)の担当校にもとづき、クエリパラメータ school_id を検証し
// AdminSchoolFilterContextKey へ絞り込み対象(nilは絞り込みなし)を格納するEcho nativeミドルウェア。
// EchoAdminAuth より後段に配置すること(AdminUserIDContextKeyに依存する)。
func EchoAdminSchoolScope(schools *services.SchoolService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			adminUserID, ok := middleware.AdminUserIDFromContext(c.Request().Context())
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
			}
			restricted, allowedSchoolIDs, err := schools.ResolveAdminAccess(adminUserID)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to resolve school access")
			}

			var schoolID *uint
			if raw := c.QueryParam("school_id"); raw != "" {
				id, err := strconv.ParseUint(raw, 10, 64)
				if err != nil {
					return echo.NewHTTPError(http.StatusBadRequest, "invalid school_id")
				}
				v := uint(id)
				schoolID = &v
			}

			if restricted {
				if schoolID == nil {
					return echo.NewHTTPError(http.StatusBadRequest, "school_id is required")
				}
				allowed := false
				for _, id := range allowedSchoolIDs {
					if id == *schoolID {
						allowed = true
						break
					}
				}
				if !allowed {
					return echo.NewHTTPError(http.StatusForbidden, "school access denied")
				}
			}

			ctx := context.WithValue(c.Request().Context(), middleware.AdminSchoolFilterContextKey, schoolID)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}
