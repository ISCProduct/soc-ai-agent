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
	companyGraph.GET("/target-year", adminCompanyGraphController.TargetYear)

	// 管理者認証必須エンドポイント
	admin := api.Group("/admin", EchoAdminAuth(userRepo, adminSecret))

	// 企業管理
	admin.GET("/companies", adminCompanyController.List)
	admin.POST("/companies", adminCompanyController.Create)
	// 軽量な企業名一覧（セレクト用）
	admin.GET("/companies/names", adminCompanyController.Names)
	admin.GET("/companies/:id", adminCompanyController.Get)
	admin.PUT("/companies/:id", adminCompanyController.Update)
	admin.POST("/companies/:id/publish", adminCompanyController.Publish)
	admin.POST("/companies/:id/reject", adminCompanyController.Reject)
	admin.GET("/companies/:id/gbiz-search", adminCompanyController.SearchGBiz)
	admin.POST("/companies/:id/gbiz-sync", adminCompanyController.SyncGBiz)
	admin.POST("/companies/:id/fetch-tech-stack", adminCompanyController.FetchTechStack)

	// クロールソース管理
	admin.GET("/crawl-sources", adminCrawlController.ListSources)
	admin.POST("/crawl-sources", adminCrawlController.CreateSource)
	admin.PUT("/crawl-sources/:id", adminCrawlController.UpdateSource)
	admin.POST("/crawl-sources/:id/run", adminCrawlController.RunSource)
	admin.GET("/crawl-runs", adminCrawlController.Runs)

	// 求人・カテゴリ管理
	admin.GET("/job-categories", adminJobController.JobCategories)
	admin.GET("/job-positions", adminJobController.JobPositions)
	admin.POST("/job-positions", adminJobController.CreateJobPosition)
	admin.Any("/job-positions/:id/:action", adminJobController.JobPositionAction)
	admin.GET("/graduate-employments", adminJobController.GraduateEmployments)
	admin.POST("/graduate-employments", adminJobController.CreateGraduateEmployment)
	admin.GET("/graduate-employments/:id", adminJobController.GetGraduateEmployment)
	admin.PUT("/graduate-employments/:id", adminJobController.UpdateGraduateEmployment)

	// ユーザー管理
	admin.GET("/users", adminUserController.List)
	admin.PUT("/users/:id", adminUserController.Update)

	// 監査ログ
	admin.GET("/audit-logs", adminAuditController.List)

	// 企業関係グラフ
	admin.POST("/company-graph/crawl", adminCompanyGraphController.Crawl)
	admin.POST("/company-graph/enrich-relations", adminCompanyGraphController.EnrichRelations)

	// 面接管理
	admin.GET("/interviews", adminInterviewController.ListSessions)
	admin.GET("/interviews/:id/videos", adminInterviewController.ListVideos)
	admin.GET("/interviews/videos/:video_id/url", adminInterviewController.VideoURL)

	// ダッシュボード
	admin.GET("/dashboard/users", adminDashboardController.ListUsers)
	admin.GET("/dashboard/users/:id", adminDashboardController.UserSessions)
	admin.GET("/dashboard/export/csv", adminDashboardController.ExportCSV)

	// コスト管理
	admin.GET("/costs/summary", adminCostsController.Summary)
	admin.GET("/costs/daily", adminCostsController.Daily)
	admin.GET("/costs/monthly", adminCostsController.Monthly)

	// プロファイル再計算
	admin.POST("/profile-recalculation", profileRecalcController.RecalculateAll)
	admin.POST("/profile-recalculation/:id", profileRecalcController.RecalculateOne)
	admin.POST("/profile-recalculation/:id/rollback", profileRecalcController.Rollback)
	admin.GET("/profile-recalculation/history", profileRecalcController.GetHistory)

	// スコアバリデーション
	admin.GET("/score-validation/correlation", scoreValidationController.GetCorrelation)
	admin.GET("/score-validation/phase-metrics", scoreValidationController.GetPhaseMetrics)
	admin.GET("/score-validation/calibration", scoreValidationController.GetCalibration)
	admin.POST("/score-validation/calibration/run", scoreValidationController.RunCalibration)
	admin.GET("/score-validation/calibration/history", scoreValidationController.GetCalibrationHistory)
	admin.GET("/score-validation/variants", scoreValidationController.ListVariants)
	admin.POST("/score-validation/variants", scoreValidationController.CreateVariant)
	admin.GET("/score-validation/variants/:id/results", scoreValidationController.GetVariantResults)

	// 集合知管理
	admin.POST("/collective-insights/rebuild-summaries", collectiveInsightController.RebuildSummaries)

	// スクレイパーセッション管理
	admin.GET("/scraper-sessions", scraperSessionController.List)
	admin.POST("/scraper-sessions", scraperSessionController.Upsert)
	admin.DELETE("/scraper-sessions/:site_key", scraperSessionController.Delete)
}
