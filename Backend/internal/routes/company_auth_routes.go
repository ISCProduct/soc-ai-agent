package routes

import (
	"Backend/internal/controllers"
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
			ctx := context.WithValue(c.Request().Context(), middleware.CompanyUserIDContextKey, companyUserID)
			ctx = context.WithValue(ctx, middleware.CompanyIDContextKey, user.CompanyID)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

func SetupCompanyAuthRoutes(
	api *echo.Group,
	authController *controllers.CompanyAuthController,
	portalController *controllers.CompanyPortalController,
	studentController *controllers.CompanyStudentController,
	companySecret string,
	users *repositories.CompanyUserRepository,
) {
	auth := api.Group("/company-auth")
	auth.POST("/login", authController.Login, echoLoginRateLimit())
	auth.POST("/accept-invite", authController.AcceptInvite, echoLoginRateLimit())
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
}
