package routes

import (
	admincontrollers "Backend/internal/controllers/admin"
	applicationcontrollers "Backend/internal/controllers/application"
	insightcontrollers "Backend/internal/controllers/insight"
	"Backend/internal/repositories"
	"Backend/internal/services/school"

	"github.com/labstack/echo/v4"
)

func SetupAdminRoutes(
	api *echo.Group,
	adminCompanyController *admincontrollers.AdminCompanyController,
	adminCrawlController *admincontrollers.AdminCrawlController,
	adminJobController *admincontrollers.AdminJobController,
	adminUserController *admincontrollers.AdminUserController,
	adminOrganizationController *admincontrollers.AdminOrganizationController,
	adminSchoolController *admincontrollers.AdminSchoolController,
	adminAuditController *admincontrollers.AdminAuditController,
	adminCompanyGraphController *admincontrollers.AdminCompanyGraphController,
	adminInterviewController *admincontrollers.AdminInterviewController,
	adminDashboardController *admincontrollers.AdminDashboardController,
	adminCostsController *admincontrollers.AdminCostsController,
	profileRecalcController *admincontrollers.AdminProfileRecalculationController,
	scoreValidationController *admincontrollers.AdminScoreValidationController,
	diagnosisQualityController *admincontrollers.AdminDiagnosisQualityController,
	collectiveInsightController *insightcontrollers.CollectiveInsightController,
	scraperSessionController *admincontrollers.AdminScraperSessionController,
	adminVectorController *admincontrollers.AdminVectorController,
	appController *applicationcontrollers.ApplicationController,
	teacherInsightController *insightcontrollers.TeacherStudentInsightController,
	userRepo *repositories.UserRepository,
	schoolService *school.SchoolService,
	adminSecret string,
) {
	admin := api.Group("/admin", EchoAdminAuth(userRepo, adminSecret))
	schoolScope := EchoAdminSchoolScope(schoolService)
	// システム管理者専用。担当校を持つ教員・学園側管理者は 403。
	platform := EchoRequirePlatformAdmin(schoolService)

	// ── 学校運営・教員向け（担当校スコープ）────────────────────────────
	admin.GET("/me/school-access", adminSchoolController.MySchoolAccess)

	admin.GET("/users", adminUserController.List, schoolScope)
	admin.PUT("/users/:id", adminUserController.Update)
	admin.DELETE("/users/:id", adminUserController.Delete)
	admin.GET("/teacher/students/tendency-analysis", teacherInsightController.TendencyAnalysis, schoolScope)

	admin.GET("/interviews", adminInterviewController.ListSessions, schoolScope)
	admin.GET("/interviews/:id/videos", adminInterviewController.ListVideos)
	admin.GET("/interviews/videos/:video_id/url", adminInterviewController.VideoURL)

	admin.GET("/dashboard/users", adminDashboardController.ListUsers, schoolScope)
	admin.GET("/dashboard/users/:id", adminDashboardController.UserSessions)
	admin.GET("/dashboard/export/csv", adminDashboardController.ExportCSV, schoolScope)

	admin.GET("/applications", appController.AdminList, schoolScope)
	admin.PATCH("/applications/:id/status", appController.AdminUpdateStatus)

	admin.GET("/graduate-employments", adminJobController.GraduateEmployments, schoolScope)
	admin.POST("/graduate-employments", adminJobController.CreateGraduateEmployment)
	admin.GET("/graduate-employments/:id", adminJobController.GetGraduateEmployment)
	admin.PUT("/graduate-employments/:id", adminJobController.UpdateGraduateEmployment)

	// 学校メンバー・企業承認は担当校の運営業務
	admin.GET("/schools/:id", adminSchoolController.Get)
	admin.POST("/schools/:id/members", adminSchoolController.AddMember)
	admin.DELETE("/schools/:id/members/:user_id", adminSchoolController.RemoveMember)
	admin.GET("/schools/:id/company-approvals", adminSchoolController.ListCompanyApprovals)
	admin.POST("/schools/:id/company-approvals", adminSchoolController.AddCompanyApproval)
	admin.DELETE("/schools/:id/company-approvals/:company_id", adminSchoolController.RemoveCompanyApproval)

	// 企業・求人カタログの閲覧と基本編集（掲載承認のため全件閲覧は担当校管理者にも必要）
	admin.GET("/companies", adminCompanyController.List)
	admin.GET("/companies/industries", adminCompanyController.Industries)
	admin.GET("/companies/names", adminCompanyController.Names)
	admin.GET("/companies/:id", adminCompanyController.Get)
	admin.PUT("/companies/:id", adminCompanyController.Update)
	// 掲載公開・非公開は全テナント共通の DataStatus / IsActive を書き換えるのでシステム管理者専用。
	// 担当校ごとの掲載可否は /schools/:id/company-approvals で扱う。
	admin.PATCH("/companies/:id/publish", adminCompanyController.Publish, platform)
	admin.PATCH("/companies/:id/reject", adminCompanyController.Reject, platform)
	admin.GET("/companies/:id/relation-graph", adminCompanyGraphController.RelationGraph)
	admin.GET("/companies/:id/interview-questions", adminInterviewController.ListCompanyQuestions)
	admin.POST("/companies/:id/interview-questions", adminInterviewController.CreateCompanyQuestion)
	admin.POST("/companies/:id/interview-questions/generate", adminInterviewController.GenerateCompanyQuestions)
	admin.PUT("/companies/:id/interview-questions/:qid", adminInterviewController.UpdateCompanyQuestion)
	admin.DELETE("/companies/:id/interview-questions/:qid", adminInterviewController.DeleteCompanyQuestion)
	admin.GET("/job-categories", adminJobController.JobCategories)
	admin.GET("/job-positions", adminJobController.JobPositions)
	admin.POST("/job-positions", adminJobController.CreateJobPosition)
	// 求人の公開・却下も企業と同じく全テナント共通の DataStatus / IsActive を書き換える。
	// 未公開企業の求人を公開できてしまうため、企業側と揃えてシステム管理者専用にする。
	admin.Any("/job-positions/:id/:action", adminJobController.JobPositionAction, platform)

	// ── システム管理者専用（テナント横断・インフラ）────────────────────
	admin.POST("/users/purge-expired", adminUserController.PurgeExpired, platform)

	admin.GET("/organizations", adminOrganizationController.List, platform)
	admin.POST("/organizations", adminOrganizationController.Create, platform)
	admin.GET("/organizations/:id", adminOrganizationController.Get, platform)
	admin.PUT("/organizations/:id", adminOrganizationController.Update, platform)
	admin.GET("/organizations/:id/members", adminOrganizationController.ListMembers, platform)
	admin.POST("/organizations/:id/members", adminOrganizationController.AddMember, platform)
	admin.PUT("/organizations/:id/members/:user_id", adminOrganizationController.UpdateMember, platform)
	admin.DELETE("/organizations/:id/members/:user_id", adminOrganizationController.RemoveMember, platform)

	admin.GET("/schools", adminSchoolController.List, platform)
	admin.POST("/schools", adminSchoolController.Create, platform)

	admin.GET("/audit-logs", adminAuditController.List, platform)

	// コスト管理
	admin.GET("/costs/summary", adminCostsController.Summary, platform)
	admin.GET("/costs/breakdown", adminCostsController.Breakdown, platform)
	admin.GET("/costs/daily", adminCostsController.Daily, platform)
	admin.GET("/costs/monthly", adminCostsController.Monthly, platform)

	admin.POST("/profile-recalculation", profileRecalcController.RecalculateAll, platform)
	admin.POST("/profile-recalculation/:id", profileRecalcController.RecalculateOne, platform)
	admin.POST("/profile-recalculation/:id/rollback", profileRecalcController.Rollback, platform)
	admin.GET("/profile-recalculation/history", profileRecalcController.GetHistory, platform)

	admin.GET("/score-validation/correlation", scoreValidationController.GetCorrelation, platform)
	admin.GET("/score-validation/phase-metrics", scoreValidationController.GetPhaseMetrics, platform)
	admin.GET("/score-validation/calibration", scoreValidationController.GetCalibration, platform)
	admin.POST("/score-validation/calibration/run", scoreValidationController.RunCalibration, platform)
	admin.GET("/score-validation/calibration/history", scoreValidationController.GetCalibrationHistory, platform)
	admin.GET("/score-validation/variants", scoreValidationController.ListVariants, platform)
	admin.POST("/score-validation/variants", scoreValidationController.CreateVariant, platform)
	admin.GET("/score-validation/variants/results", scoreValidationController.GetVariantResults, platform)
	admin.GET("/diagnosis-quality", diagnosisQualityController.List, platform)

	admin.POST("/collective-insights/rebuild-summaries", collectiveInsightController.RebuildSummaries, platform)

	admin.GET("/scraper-sessions", scraperSessionController.List, platform)
	admin.POST("/scraper-sessions", scraperSessionController.Upsert, platform)
	admin.DELETE("/scraper-sessions/:site_key", scraperSessionController.Delete, platform)

	admin.GET("/vector/status", adminVectorController.Status, platform)
	admin.POST("/vector/reembed", adminVectorController.Reembed, platform)
	admin.GET("/vector/stats", adminVectorController.Stats, platform)
	admin.GET("/vector/collections", adminVectorController.Collections, platform)

	admin.GET("/crawl-sources", adminCrawlController.ListSources, platform)
	admin.POST("/crawl-sources", adminCrawlController.CreateSource, platform)
	admin.PUT("/crawl-sources/:id", adminCrawlController.UpdateSource, platform)
	admin.POST("/crawl-sources/:id/run", adminCrawlController.RunSource, platform)
	admin.GET("/crawl-runs", adminCrawlController.Runs, platform)

	// /admin 配下に認証なしのグループを作らない(#1411)。年度計算だけを返す軽い
	// エンドポイントだが、認証の外にあると「/admin に生やせば守られる」前提が崩れ、
	// 次にこのグループへ足したハンドラが無認証で公開される。
	admin.GET("/company-graph/target-year", adminCompanyGraphController.TargetYear)
	admin.POST("/company-graph/crawl", adminCompanyGraphController.Crawl, platform)
	admin.POST("/company-graph/enrich-relations", adminCompanyGraphController.EnrichRelations, platform)

	admin.POST("/companies", adminCompanyController.Create, platform)
	admin.GET("/companies/l1-coverage", adminCompanyController.GetL1Coverage, platform)
	admin.POST("/companies/warm-l1", adminCompanyController.WarmL1Catalog, platform)
	admin.POST("/companies/fetch-missing-batch", adminCompanyController.FetchMissingBatch, platform)
	admin.POST("/companies/seed-l1", adminCompanyController.SeedL1Catalog, platform)
	admin.POST("/companies/web-search", adminCompanyController.WebSearchCompanyInfo, platform)
	admin.POST("/companies/web-search-relations", adminCompanyController.WebSearchCompanyRelations, platform)
	admin.GET("/companies/:id/gbiz-search", adminCompanyController.SearchGBiz, platform)
	admin.POST("/companies/:id/gbiz-sync", adminCompanyController.SyncGBiz, platform)
	admin.POST("/companies/:id/fetch-tech-stack", adminCompanyController.FetchTechStack, platform)
	admin.POST("/companies/:id/fetch-info", adminCompanyController.FetchCompanyInfo, platform)
	admin.POST("/companies/:id/confirm-info", adminCompanyController.ConfirmCompanyInfo, platform)
	admin.POST("/companies/:id/fetch-relations", adminCompanyController.FetchCompanyRelations, platform)
	admin.POST("/companies/:id/confirm-relations", adminCompanyController.ConfirmCompanyRelations, platform)
	admin.POST("/companies/:id/fetch-jobs", adminCompanyController.FetchJobs, platform)
	admin.POST("/companies/:id/fetch-persona", adminCompanyController.FetchPersona, platform)
	admin.POST("/companies/:id/fetch-all", adminCompanyController.FetchAllMissing, platform)
	admin.POST("/companies/:id/fetch-primary", adminCompanyController.FetchPrimary, platform)
}
