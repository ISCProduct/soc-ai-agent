package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/middleware"
	"Backend/internal/repositories"
	"net/http"
)

func SetupAdminRoutes(
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
	auth := func(f http.HandlerFunc) http.HandlerFunc {
		return middleware.AdminAuthFunc(userRepo, adminSecret, f)
	}

	http.HandleFunc("/api/admin/companies", auth(adminCompanyController.ListOrCreate))
	// http.HandleFunc("/api/admin/companies/search-gbiz", auth(adminCompanyController.SearchGBizRoute)) // gBizINFO停止中
	http.HandleFunc("/api/admin/companies/", auth(adminCompanyController.Detail))
	http.HandleFunc("/api/admin/crawl-sources", auth(adminCrawlController.Sources))
	http.HandleFunc("/api/admin/crawl-sources/", auth(adminCrawlController.SourceDetail))
	http.HandleFunc("/api/admin/crawl-runs", auth(adminCrawlController.Runs))
	http.HandleFunc("/api/admin/job-categories", auth(adminJobController.JobCategories))
	http.HandleFunc("/api/admin/job-positions", auth(adminJobController.JobPositions))
	http.HandleFunc("/api/admin/job-positions/", auth(adminJobController.JobPositionAction))
	http.HandleFunc("/api/admin/graduate-employments", auth(adminJobController.GraduateEmployments))
	http.HandleFunc("/api/admin/graduate-employments/", auth(adminJobController.GraduateEmploymentDetail))
	http.HandleFunc("/api/admin/users", auth(adminUserController.List))
	http.HandleFunc("/api/admin/users/", auth(adminUserController.Update))
	http.HandleFunc("/api/admin/audit-logs", auth(adminAuditController.List))

	// Company graph (scraping pipeline)
	http.HandleFunc("/api/admin/company-graph/target-year", adminCompanyGraphController.TargetYear)
	http.HandleFunc("/api/admin/company-graph/crawl", auth(adminCompanyGraphController.Crawl))

	// Interview management
	http.HandleFunc("/api/admin/interviews", auth(adminInterviewController.ListSessions))
	http.HandleFunc("/api/admin/interviews/", auth(adminInterviewController.Route))

	// Dashboard
	http.HandleFunc("/api/admin/dashboard/users", auth(adminDashboardController.ListUsers))
	http.HandleFunc("/api/admin/dashboard/users/", auth(adminDashboardController.UserSessions))
	http.HandleFunc("/api/admin/dashboard/export/csv", auth(adminDashboardController.ExportCSV))

	// API Cost monitoring
	http.HandleFunc("/api/admin/costs/summary", auth(adminCostsController.Summary))
	http.HandleFunc("/api/admin/costs/daily", auth(adminCostsController.Daily))
	http.HandleFunc("/api/admin/costs/monthly", auth(adminCostsController.Monthly))

	// Profile recalculation
	http.HandleFunc("/api/admin/profile-recalculation", auth(profileRecalcController.Route))
	http.HandleFunc("/api/admin/profile-recalculation/", auth(profileRecalcController.Route))

	// Score validation (correlation, calibration, A/B test)
	http.HandleFunc("/api/admin/score-validation/", auth(scoreValidationController.Route))

	// Collective insight batch
	http.HandleFunc("/api/admin/collective-insights/rebuild-summaries", auth(collectiveInsightController.RebuildSummaries))

	// Scraper session management
	http.HandleFunc("/api/admin/scraper-sessions", auth(scraperSessionController.Sessions))
	http.HandleFunc("/api/admin/scraper-sessions/", auth(scraperSessionController.SessionDetail))
}
