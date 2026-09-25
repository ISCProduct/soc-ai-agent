package routes

import (
	companycontrollers "Backend/internal/controllers/company"
	"Backend/internal/middleware"
	"Backend/internal/repositories"

	"context"

	"github.com/labstack/echo/v4"
)

// EchoCompanyAuth は X-Company-User-Token JWT を検証し、企業ユーザーIDと company_id をコンテキストへ載せる。
func EchoCompanyAuth(companySecret string, users *repositories.CompanyUserRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if companySecret == "" {
				return echo.NewHTTPError(503, "Service Unavailable: company authentication not configured")
			}
			token := c.Request().Header.Get("X-Company-User-Token")
			if token == "" {
				return echo.NewHTTPError(401, "Unauthorized")
			}
			companyUserID, _, err := middleware.ParseJWT(token, companySecret)
			if err != nil {
				return echo.NewHTTPError(401, "Unauthorized")
			}
			if users == nil {
				return echo.NewHTTPError(503, "Service Unavailable: company authentication not configured")
			}
			user, err := users.FindByID(companyUserID)
			if err != nil {
				return echo.NewHTTPError(500, "failed to resolve company user")
			}
			if user == nil || !user.PasswordSet() {
				return echo.NewHTTPError(401, "Unauthorized")
			}
			// 無効化されたアカウントは、JWTの有効期限が切れる前でもここで弾く（#1196）。
			if user.Disabled() {
				return echo.NewHTTPError(403, "このアカウントは無効化されています")
			}
			ctx := context.WithValue(c.Request().Context(), middleware.CompanyUserIDContextKey, companyUserID)
			ctx = context.WithValue(ctx, middleware.CompanyIDContextKey, user.CompanyID)
			// 破壊的操作を owner に限るため、役割もここで載せる（#1319）。
			// ハンドラごとに引き直すと、引き忘れた経路だけ権限判定が抜ける。
			ctx = context.WithValue(ctx, middleware.CompanyUserRoleContextKey, user.Role)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

func SetupCompanyAuthRoutes(
	api *echo.Group,
	authController *companycontrollers.CompanyAuthController,
	portalController *companycontrollers.CompanyPortalController,
	studentController *companycontrollers.CompanyStudentController,
	applicationController *companycontrollers.CompanyPortalApplicationController,
	jobController *companycontrollers.CompanyPortalJobController,
	profileController *companycontrollers.CompanyPortalProfileController,
	companySecret string,
	users *repositories.CompanyUserRepository,
) {
	auth := api.Group("/company-auth")
	auth.POST("/login", authController.Login, echoLoginRateLimit())
	auth.POST("/accept-invite", authController.AcceptInvite, echoLoginRateLimit())
	// パスワードリセット（#1196）。総当たりとメール爆撃を防ぐためレート制限をかける。
	auth.POST("/forgot-password", authController.ForgotPassword, echoPasswordResetRateLimit())
	auth.POST("/reset-password", authController.ResetPassword, echoLoginRateLimit())
	auth.POST("/refresh", authController.Refresh)
	auth.POST("/logout", authController.Logout)

	protected := api.Group("/company-auth", EchoCompanyAuth(companySecret, users))
	protected.GET("/me", authController.Me)

	portal := api.Group("/company-portal", EchoCompanyAuth(companySecret, users))
	portal.GET("/companies/:id", portalController.GetCompany)

	// 学生検索・タグ管理 (#1094)。company_id はJWT由来のため、
	// 他社データへ越境するクエリパラメータは受け付けない。
	portal.GET("/students", studentController.List)
	portal.POST("/students/semantic-search", studentController.SemanticSearch)
	portal.GET("/students/:userID", studentController.Detail)
	portal.POST("/students/:userID/tags", studentController.AddTag)
	portal.DELETE("/students/:userID/tags/:tagID", studentController.RemoveTag)
	portal.GET("/tags", studentController.ListTags)
	portal.GET("/industries", studentController.Industries)

	// ダッシュボードと応募者管理 (#1320)。
	// company_id はJWT由来。他社の応募IDを指定された場合は 403 を返す。
	if applicationController != nil {
		portal.GET("/dashboard", applicationController.Dashboard)
		portal.GET("/applications", applicationController.List)
		// 選考ステータスの変更は破壊的操作なので owner のみ（コントローラ側で判定）。
		portal.PATCH("/applications/:id/status", applicationController.UpdateStatus)
	}

	// 求人管理 (#1321)。一覧は全員、作成・編集・公開は owner のみ
	// （コントローラ側で判定）。削除は提供しない（応募が紐づくため非公開化で対応）。
	if jobController != nil {
		portal.GET("/jobs", jobController.List)
		portal.POST("/jobs", jobController.Create)
		portal.PATCH("/jobs/:id", jobController.Update)
		portal.POST("/jobs/:id/publish", jobController.Publish)
	}

	// 自社プロフィール編集と担当者管理 (#1322)。
	// 管理者向けは :id で企業を指定するが、ここでは自社しか触れないため
	// パラメータを持たせない。更新系は owner のみ（コントローラ側で判定）。
	if profileController != nil {
		portal.GET("/company", profileController.GetCompany)
		portal.PATCH("/company", profileController.UpdateCompany)
		portal.GET("/members", profileController.ListMembers)
		portal.POST("/members", profileController.InviteMember)
		portal.PATCH("/members/:userID", profileController.SetMemberDisabled)
	}
}
