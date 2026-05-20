package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/repositories"

	"github.com/labstack/echo/v4"
)

func SetupAdminRoutes(
	api *echo.Group,
	adminCompanyController *controllers.AdminCompanyController,
	adminCrawlController *controllers.AdminCrawlController,
	adminJobController *controllers.AdminJobController,
	adminUserController *controllers.AdminUserController,
	adminAuditController *controllers.AdminAuditController,
	adminCompanyGraphController *controllers.AdminCompanyGraphController,
	adminInterviewController *controllers.AdminInterviewController,
	adminDashboardController *controllers.AdminDashboardController,
	adminCostsController *controllers.AdminCostsController,
	profileRecalcController *controllers.AdminProfileRecalculationController,
	scoreValidationController *controllers.AdminScoreValidationController,
	collectiveInsightController *controllers.CollectiveInsightController,
	scraperSessionController *controllers.AdminScraperSessionController,
	userRepo *repositories.UserRepository,
	adminSecret string,
) {
	// 認証不要（公開）エンドポイント
	companyGraph := api.Group("/admin/company-graph")
	companyGraph.Any("/target-year", wrap(adminCompanyGraphController.TargetYear))

	// 管理者認証必須エンドポイント
	admin := api.Group("/admin", EchoAdminAuth(userRepo, adminSecret))

	admin.Any("/companies", wrap(adminCompanyController.ListOrCreate))
	admin.Any("/companies/*", wrap(adminCompanyController.Detail))

	admin.Any("/crawl-sources", wrap(adminCrawlController.Sources))
	admin.Any("/crawl-sources/*", wrap(adminCrawlController.SourceDetail))
	admin.Any("/crawl-runs", wrap(adminCrawlController.Runs))

	admin.Any("/job-categories", wrap(adminJobController.JobCategories))
	admin.Any("/job-positions", wrap(adminJobController.JobPositions))
	admin.Any("/job-positions/*", wrap(adminJobController.JobPositionAction))
	admin.Any("/graduate-employments", wrap(adminJobController.GraduateEmployments))
	admin.Any("/graduate-employments/*", wrap(adminJobController.GraduateEmploymentDetail))

	admin.Any("/users", wrap(adminUserController.List))
	admin.Any("/users/*", wrap(adminUserController.Update))

	admin.Any("/audit-logs", wrap(adminAuditController.List))

	admin.Any("/company-graph/crawl", wrap(adminCompanyGraphController.Crawl))

	admin.Any("/interviews", wrap(adminInterviewController.ListSessions))
	admin.Any("/interviews/*", wrap(adminInterviewController.Route))

	admin.Any("/dashboard/users", wrap(adminDashboardController.ListUsers))
	admin.Any("/dashboard/users/*", wrap(adminDashboardController.UserSessions))
	admin.Any("/dashboard/export/csv", wrap(adminDashboardController.ExportCSV))

	admin.Any("/costs/summary", wrap(adminCostsController.Summary))
	admin.Any("/costs/daily", wrap(adminCostsController.Daily))
	admin.Any("/costs/monthly", wrap(adminCostsController.Monthly))

	admin.Any("/profile-recalculation", wrap(profileRecalcController.Route))
	admin.Any("/profile-recalculation/*", wrap(profileRecalcController.Route))

	admin.Any("/score-validation/*", wrap(scoreValidationController.Route))

	admin.Any("/collective-insights/rebuild-summaries", wrap(collectiveInsightController.RebuildSummaries))

	admin.Any("/scraper-sessions", wrap(scraperSessionController.Sessions))
	admin.Any("/scraper-sessions/*", wrap(scraperSessionController.SessionDetail))
}
