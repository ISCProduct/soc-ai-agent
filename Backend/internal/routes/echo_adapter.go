package routes

import (
	"Backend/internal/middleware"
	"Backend/internal/repositories"
	"Backend/internal/services"
	"Backend/internal/services/auth"
	"Backend/internal/services/organization"
	"Backend/internal/usagectx"
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

// OrganizationIDResolver は認証済みユーザーの組織IDを解決する。
type OrganizationIDResolver interface {
	ResolveOrganizationID(userID uint) (uint, error)
	IsUserAdmin(userID uint) (bool, error)
}

// EchoUserAuth はX-User-Token JWTを検証するEcho nativeミドルウェアを返す。
// userSecret が未設定の場合はフェイルクローズ（503）として動作する。
// access が非 nil の場合、退会済みユーザーを遮断する。
// orgs が非 nil の場合、organization_id をリクエストコンテキストへ載せる。
func EchoUserAuth(userSecret string, access auth.UserAccessGuard, orgs ...OrganizationIDResolver) echo.MiddlewareFunc {
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
					if errors.Is(err, auth.ErrAccountWithdrawn) {
						return echo.NewHTTPError(http.StatusForbidden, "account has been withdrawn")
					}
					return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
				}
			}
			ctx := context.WithValue(c.Request().Context(), middleware.UserIDContextKey, userID)
			if resolver != nil {
				orgID, err := resolver.ResolveOrganizationID(userID)
				if err != nil {
					if errors.Is(err, organization.ErrOrganizationDisabled) {
						return echo.NewHTTPError(http.StatusForbidden, "organization is disabled")
					}
					return echo.NewHTTPError(http.StatusInternalServerError, "failed to resolve organization")
				}
				if orgID == 0 {
					return echo.NewHTTPError(http.StatusForbidden, "organization not found")
				}
				ctx = context.WithValue(ctx, middleware.OrganizationIDContextKey, orgID)
				if tenantOrgID, ok := middleware.TenantOrganizationIDFromContext(ctx); ok && tenantOrgID != orgID {
					isAdmin, err := resolver.IsUserAdmin(userID)
					if err != nil {
						return echo.NewHTTPError(http.StatusInternalServerError, "failed to resolve admin status")
					}
					if !isAdmin {
						return echo.NewHTTPError(http.StatusForbidden, "tenant mismatch")
					}
					// プラットフォーム管理者は自身の所属組織に関わらずアクセス先の学園として扱う
					ctx = context.WithValue(ctx, middleware.OrganizationIDContextKey, tenantOrgID)
				}
			}
			// AI利用量の配賦先をここで一度だけ載せる（#1294）。
			// 各サービスが個別にユーザー/組織を引き回さなくても、この経路の
			// AI 呼び出しはすべて機能別・組織別に配賦できる。
			orgID, _ := middleware.OrganizationIDFromContext(ctx)
			ctx = usagectx.WithActor(ctx, userID, orgID)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// EchoTenantResolver は X-Tenant-Slug ヘッダー(学園サブドメインラベル)から組織を解決し、
// リクエストコンテキストへ載せるEcho nativeミドルウェア。
// ヘッダー未指定の場合は何もしない（テナント概念のないアクセスとの後方互換のため）。
func EchoTenantResolver(orgs *organization.OrganizationService) echo.MiddlewareFunc {
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
			// 署名・有効期限・管理者1名単位の失効(admin_token_not_before)を検証する(#1155)
			switch err := middleware.ValidateAdminTokenForUser(token, user, adminSecret); {
			case errors.Is(err, middleware.ErrAdminTokenExpired):
				return echo.NewHTTPError(http.StatusForbidden, "管理者トークンの有効期限が切れました。再ログインしてください。")
			case err != nil:
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

// echoGuestAIRateLimit は未認証で叩けるAI呼び出し（ES添削・企業WEB検索）の
// コスト濫用を止めるレート制限ミドルウェア（#1154）。
// 認証を付けられない仕様のため、IP単位＋全体上限の二段で課金の総量を抑える。
//
// ponytail: 制限器はタスク内メモリ（prodのREDIS_URLもlocalhostサイドカーで同スコープ）。
// backend を複数タスクへ増やす場合は共有Redisの KeyRateLimiter へ差し替えること。
func echoGuestAIRateLimit() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := middleware.GetClientIP(c.Request())
			if !middleware.GuestAIRateLimiter.Allow(ip) || !middleware.GuestAIGlobalRateLimiter.Allow("global") {
				return echo.NewHTTPError(http.StatusTooManyRequests, "Too Many Requests: リクエスト上限に達しました。しばらく待ってから再試行してください。")
			}
			return next(c)
		}
	}
}

// EchoMetricsAuth は /metrics を Bearer トークンで保護するミドルウェア（#1186）。
//
// backend の ALB はインターネットに直結しているため、無防備に開けるとエンドポイント一覧・
// リクエスト数・レイテンシ分布が誰でも読める。Prometheus の scrape_config は
// authorization.credentials で Bearer を送れるので、標準的な形で合わせる。
func EchoMetricsAuth(token string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// "Bearer " 無しの素のトークンは受け付けない。TrimPrefix だと素通りしてしまう。
			provided, ok := strings.CutPrefix(c.Request().Header.Get("Authorization"), "Bearer ")
			// 比較時間から推測されないよう定数時間比較を使う（他の認証経路と同じ方針）。
			if !ok || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
				return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
			}
			return next(c)
		}
	}
}

// MetricsSkipper は /metrics の計装対象から外すリクエストを判定する（#1186）。
//
// ALB のヘルスチェックは30秒ごとに来るため、含めるとリクエスト数の大半を占めて
// 実際のトラフィックが読めなくなる。
//
// ルートに一致しないリクエスト(404)も外す。この場合 c.Path() が空になり、
// echoprometheus は url ラベルへ生のパスを入れる。ALB はインターネット直結で
// スキャンを日常的に受けるため、放置するとラベルの種類が無限に増え、プロセスと
// スクレイパのメモリを食いつぶす。404 の総数は ALB 側のメトリクスで見る。
func MetricsSkipper(c echo.Context) bool {
	switch c.Path() {
	case "", "/health", "/healthz", "/metrics":
		return true
	}
	return false
}
