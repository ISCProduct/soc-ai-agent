package routes

import (
	"context"
	"errors"
	"net/http"

	companycontrollers "Backend/internal/controllers/company"
	"Backend/internal/middleware"
	"Backend/internal/repositories"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
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

// EchoRequireVerifiedCompany は審査前の企業が学生情報を見たりスカウトを送ったりできないようにする。
//
// 公開登録は審査前でもJWTを返す（プロフィール整備のため）。学生検索・分析・スカウトは
// companies.is_verified が true になるまで拒否する。企業が見つからない場合も同じ403にする。
func EchoRequireVerifiedCompany(companies *repositories.CompanyRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if companies == nil {
				return echo.NewHTTPError(http.StatusServiceUnavailable, "Service Unavailable: company verification not configured")
			}
			companyID, ok := middleware.CompanyIDFromContext(c.Request().Context())
			if !ok || companyID == 0 {
				return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
			}
			company, err := companies.FindByID(companyID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return echo.NewHTTPError(http.StatusForbidden, "企業の審査が完了するまで学生情報の閲覧とスカウト送信はできません")
				}
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to resolve company")
			}
			if company == nil || !company.IsVerified {
				return echo.NewHTTPError(http.StatusForbidden, "企業の審査が完了するまで学生情報の閲覧とスカウト送信はできません")
			}
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
	schoolAppController *companycontrollers.CompanyPortalSchoolApplicationController,
	scoutController *companycontrollers.CompanyPortalScoutController,
	companySecret string,
	users *repositories.CompanyUserRepository,
	companies *repositories.CompanyRepository,
) {
	auth := api.Group("/company-auth")
	auth.POST("/login", authController.Login, echoLoginRateLimit())
	auth.POST("/register", authController.Register, echoLoginRateLimit())
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

	// 学生検索・分析とスカウトは審査完了まで開けない。
	// 求人や自社プロフィールは審査前でも編集できる。
	verified := portal.Group("", EchoRequireVerifiedCompany(companies))
	verified.GET("/students", studentController.List)
	verified.POST("/students/semantic-search", studentController.SemanticSearch)
	verified.GET("/students/:userID", studentController.Detail)
	verified.POST("/students/:userID/tags", studentController.AddTag)
	verified.DELETE("/students/:userID/tags/:tagID", studentController.RemoveTag)

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

	// 求人管理 (#1321)。一覧は全員、作成・編集・公開・削除は owner のみ
	// （コントローラ側で判定）。削除は論理削除で、紐づく参照は残す。
	if jobController != nil {
		portal.GET("/jobs", jobController.List)
		portal.POST("/jobs", jobController.Create)
		portal.PATCH("/jobs/:id", jobController.Update)
		portal.DELETE("/jobs/:id", jobController.Delete)
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

	// 学校への掲載申請 (#1506)。一覧は全員、申請・取消は owner のみ
	// （コントローラ側で判定）。company_id はJWT由来。
	if schoolAppController != nil {
		portal.GET("/school-applications", schoolAppController.List)
		portal.POST("/school-applications", schoolAppController.Create)
		portal.DELETE("/school-applications/:id", schoolAppController.Delete)
	}

	// スカウト送信・テンプレート管理 (#1095)。company_id はJWT由来。
	// 審査前の企業には学生への送信手段を開けない。
	if scoutController != nil {
		verified.GET("/scout-templates", scoutController.ListTemplates)
		verified.POST("/scout-templates", scoutController.CreateTemplate)
		verified.PATCH("/scout-templates/:id", scoutController.UpdateTemplate)
		verified.DELETE("/scout-templates/:id", scoutController.DeleteTemplate)
		verified.GET("/scouts", scoutController.List)
		verified.POST("/scouts", scoutController.Send)
		verified.GET("/scouts/cooldown", scoutController.Cooldown)
	}
}
