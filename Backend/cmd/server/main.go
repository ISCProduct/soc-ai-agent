package main

import (
	"Backend/internal/config"
	"Backend/internal/controllers"
	"Backend/internal/infrastructure/redisx"
	"Backend/internal/logger"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/queue"
	"Backend/internal/repositories"
	"Backend/internal/routes"
	"Backend/internal/scraper"
	"Backend/internal/services"
	"Backend/internal/services/admin"
	"Backend/internal/services/analysis"
	"Backend/internal/services/application"
	"Backend/internal/services/auth"
	"Backend/internal/services/chat"
	"Backend/internal/services/company"
	"Backend/internal/services/costs"
	"Backend/internal/services/discord"
	"Backend/internal/services/email"
	"Backend/internal/services/flywheel"
	"Backend/internal/services/gbizinfo"
	"Backend/internal/services/github"
	"Backend/internal/services/interview"
	"Backend/internal/services/matching"
	"Backend/internal/services/oauth"
	"Backend/internal/services/organization"
	"Backend/internal/services/refreshtoken"
	"Backend/internal/services/resume"
	"Backend/internal/services/schedule"
	"Backend/internal/services/shared"
	"Backend/internal/services/skillscore"
	"Backend/internal/services/storage"
	"Backend/migrations"
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// wildcardPattern は "https://*.shukatsu-ai.jp" のようなオリジンパターンの前後を保持する。
type wildcardPattern struct {
	prefix string
	suffix string
}

func (p wildcardPattern) matches(origin string) bool {
	return len(origin) >= len(p.prefix)+len(p.suffix) &&
		strings.HasPrefix(origin, p.prefix) && strings.HasSuffix(origin, p.suffix)
}

func buildAllowedOrigins() (exact map[string]struct{}, wildcards []wildcardPattern) {
	exact = make(map[string]struct{})
	raw := os.Getenv("ALLOWED_ORIGINS")

	for _, origin := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			continue
		}
		// "https://*.shukatsu-ai.jp" のようなワイルドカードエントリは学園マルチテナント
		// サブドメイン(<学園slug>.shukatsu-ai.jp)やadmin.shukatsu-ai.jpからの直接アクセスを許可する。
		if idx := strings.IndexByte(trimmed, '*'); idx != -1 {
			wildcards = append(wildcards, wildcardPattern{prefix: trimmed[:idx], suffix: trimmed[idx+1:]})
			continue
		}
		exact[trimmed] = struct{}{}
	}

	// フェイルセーフ: ALLOWED_ORIGINS 未設定時は全オリジン拒否（#327）
	// ローカル開発時は .env に ALLOWED_ORIGINS=http://localhost:3000 を明示設定してください。
	if len(exact) == 0 && len(wildcards) == 0 {
		slog.Warn("ALLOWED_ORIGINS が未設定のため、全クロスオリジンリクエストを拒否します",
			"hint", "ローカル開発時は .env に ALLOWED_ORIGINS=http://localhost:3000 を設定してください")
	}

	return exact, wildcards
}

func isAllowedOrigin(origin string, exact map[string]struct{}, wildcards []wildcardPattern) bool {
	if _, ok := exact[origin]; ok {
		return true
	}
	for _, w := range wildcards {
		if w.matches(origin) {
			return true
		}
	}
	return false
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// buildCORSMiddleware は許可オリジンを一度だけ構築してCORSミドルウェアを返す
func buildCORSMiddleware() func(http.Handler) http.Handler {
	exact, wildcards := buildAllowedOrigins()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			allowed := isAllowedOrigin(origin, exact, wildcards)

			w.Header().Add("Vary", "Origin")
			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				// credentials: 'include' 付きのフロント直叩き（Googleカレンダー OAuth など）に必要
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, X-User-ID, X-User-Token, X-Admin-Email, X-Admin-Token")

			if r.Method == "OPTIONS" {
				if origin != "" && !allowed {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// checkAnnotationFont はサーバー起動時に PDF アノテーション用フォントの存在を確認し、
// 設定に問題がある場合は警告ログを出力する。
// フォントが存在しない場合もサーバー起動は継続するが、PDF 注釈が劣化する旨を明示する。
func checkAnnotationFont() {
	fontPath := os.Getenv("ANNOTATION_FONT_PATH")
	if fontPath == "" {
		slog.Warn("ANNOTATION_FONT_PATH が設定されていません。フォールバックフォントを使用します",
			"hint", "環境変数 ANNOTATION_FONT_PATH にフォントパスを設定してください")
		return
	}
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		slog.Warn("ANNOTATION_FONT_PATH のフォントが見つかりません",
			"path", fontPath,
			"hint", "Dockerfileで fonts-noto-cjk がインストールされているか確認してください")
		return
	}
	slog.Info("PDF アノテーションフォント確認済み", "path", fontPath)
}

func main() {
	// 構造化ログの初期化（LOG_LEVEL / LOG_FORMAT 環境変数で制御）
	logger.Setup()

	// PDF アノテーションフォントの存在チェック（起動時警告）
	checkAnnotationFont()

	// 設定を読み込む（環境変数の読み込みはconfig.LoadConfig内で実施）
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// データベース接続
	db, err := config.ConnectDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// マイグレーション実行（バージョン管理型・多重起動は GET_LOCK で排他される #614）
	if err := migrations.Up(cfg.DSN()); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	slog.Info("Database migration completed")

	// 初期データ投入
	if err := models.SeedData(db); err != nil {
		log.Fatalf("Failed to seed data: %v", err)
	}
	slog.Info("Database seeding completed")

	// OpenAI クライアント初期化
	aiClient, err := openai.NewFromEnv("")
	if err != nil {
		log.Fatalf("Failed to initialize OpenAI client: %v", err)
	}

	// OAuth設定読み込み
	oauthConfig := config.LoadOAuthConfig()

	// ── インフラ層: リポジトリ (domain/repository インターフェースを実装) ──────────
	// ユーザー・認証
	userRepo := repositories.NewUserRepository(db)
	pendingRegistrationRepo := repositories.NewPendingRegistrationRepository(db)
	// チャット・分析
	questionWeightRepo := repositories.NewQuestionWeightRepository(db)
	chatMessageRepo := repositories.NewChatMessageRepository(db)
	userWeightScoreRepo := repositories.NewUserWeightScoreRepository(db)
	aiGeneratedQuestionRepo := repositories.NewAIGeneratedQuestionRepository(db)
	predefinedQuestionRepo := repositories.NewPredefinedQuestionRepository(db)
	phaseRepo := repositories.NewAnalysisPhaseRepository(db)
	progressRepo := repositories.NewUserAnalysisProgressRepository(db)
	sessionValidationRepo := repositories.NewSessionValidationRepository(db)
	conversationContextRepo := repositories.NewConversationContextRepository(db)
	// 職種・企業
	jobCategoryRepo := repositories.NewJobCategoryRepository(db)
	companyRepo := repositories.NewCompanyRepository(db)
	crawlRepo := repositories.NewCrawlRepository(db)
	popularityRepo := repositories.NewCompanyPopularityRepository(db)
	scraperSessionRepo := repositories.NewScraperSessionRepository(db)
	graduateRepo := repositories.NewGraduateEmploymentRepository(db)
	companyRelationRepo := repositories.NewCompanyRelationRepository(db)
	companyQueryRepo := repositories.NewCompanyQueryRepository(db)
	matchRepo := repositories.NewUserCompanyMatchRepository(db)
	profileRecalcRepo := repositories.NewProfileRecalculationRepository(db)
	// 埋め込み・マッチング
	userEmbeddingRepo := repositories.NewUserEmbeddingRepository(db)
	jobEmbeddingRepo := repositories.NewJobCategoryEmbeddingRepository(db)
	// 面接
	interviewSessionRepo := repositories.NewInterviewSessionRepository(db)
	interviewUtteranceRepo := repositories.NewInterviewUtteranceRepository(db)
	interviewReportRepo := repositories.NewInterviewReportRepository(db)
	videoRepo := repositories.NewInterviewVideoRepository(db)
	interviewCompanyQuestionRepo := repositories.NewInterviewCompanyQuestionRepository(db)
	interviewQuestionStateRepo := repositories.NewInterviewQuestionStateRepository(db)
	// その他
	resumeRepo := repositories.NewResumeRepository(db)
	auditLogRepo := repositories.NewAuditLogRepository(db)
	// GitHub連携
	githubRepo := repositories.NewGitHubRepository(db)
	skillScoreRepo := repositories.NewSkillScoreRepository(db)
	// 応募・選考ステータス
	appStatusRepo := repositories.NewUserApplicationStatusRepository(db)
	// APIコストモニタリング
	apiCallLogRepo := repositories.NewAPICallLogRepository(db)
	realtimeUsageRepo := repositories.NewRealtimeUsageRepository(db)

	// サービス層の初期化
	emailService := email.NewEmailService()

	// Redis（#617）: レート制限共有 + 永続ジョブキュー。未設定時はインメモリ/go func フォールバック。
	rdb := redisx.NewFromEnv()
	if rdb != nil {
		middleware.ConfigureRateLimiters(
			middleware.NewRedisRateLimiter(rdb, "login", time.Minute, 20),
			middleware.NewRedisRateLimiter(rdb, "password_reset", time.Hour, 5),
		)
	}
	var jobEnqueuer shared.JobEnqueuer
	var queueServer *queue.Server
	if rdb != nil {
		qClient := queue.NewClient(rdb)
		jobEnqueuer = &queue.EnqueuerAdapter{Client: qClient}
	}

	apiCostService := costs.NewAPICostService(apiCallLogRepo)
	realtimeUsageService := costs.NewRealtimeUsageService(realtimeUsageRepo, emailService)
	companySearchBudget := costs.NewCompanySearchBudgetService(apiCallLogRepo, emailService)
	companySearchFlight := company.NewCompanySearchFlight()
	// OpenAI APIコール時にトークン使用量をロギング
	aiClient.OnUsage = func(model string, promptTokens, completionTokens int) {
		apiCostService.LogCall(model, promptTokens, completionTokens)
	}
	schoolRepo := repositories.NewSchoolRepository(db)
	schoolService := services.NewSchoolService(schoolRepo)
	authService := auth.NewAuthService(userRepo, pendingRegistrationRepo, emailService)
	authService.SetDB(db)
	authService.SetSchoolRepo(schoolRepo)
	if jobEnqueuer != nil {
		authService.SetJobEnqueuer(jobEnqueuer)
	}
	// リフレッシュトークン管理 (#616)
	refreshTokenRepo := repositories.NewUserRefreshTokenRepository(db)
	refreshTokenService := refreshtoken.NewRefreshTokenService(refreshTokenRepo)
	authService.SetRefreshTokenService(refreshTokenService)
	skillScoreService := skillscore.NewSkillScoreService(skillScoreRepo)
	githubService := github.NewGitHubService(githubRepo, skillScoreService, aiClient)
	oauthService := oauth.NewOAuthService(userRepo, oauthConfig, githubService)
	oauthService.SetRefreshTokenService(refreshTokenService)
	chatService := chat.NewChatService(aiClient, questionWeightRepo, chatMessageRepo, userWeightScoreRepo, aiGeneratedQuestionRepo, predefinedQuestionRepo, jobCategoryRepo, userRepo, userEmbeddingRepo, jobEmbeddingRepo, phaseRepo, progressRepo, sessionValidationRepo, conversationContextRepo)
	questionService := chat.NewQuestionGeneratorService(aiClient, questionWeightRepo)
	matchingService := matching.NewMatchingService(userWeightScoreRepo, companyRepo, matchRepo, aiClient)
	resumeService := resume.NewResumeService(resumeRepo, "storage/resumes", aiClient)
	crawlService := company.NewCrawlService(crawlRepo, companyRepo, popularityRepo, aiClient)
	gbizInfoRepo := repositories.NewGBizInfoRepository(db)
	gbizInfoService := gbizinfo.NewGBizInfoService(cfg, gbizInfoRepo, companyRepo, companyRelationRepo)
	infoFetcher := company.NewCompanyInfoFetcher(companyRepo, aiClient, gbizInfoService)
	infoFetcher.SetSearchBudget(companySearchBudget)
	infoFetcher.SetSearchFlight(companySearchFlight)
	// 関連企業として新規作成された会社(gbizinfo経由/AI検索経由の両方)にも
	// infoFetcherで詳細情報を充填する(空データの企業が量産される問題への対応)。
	gbizInfoService.SetDetailFetcher(infoFetcher)
	relationsFetcher := company.NewCompanyRelationsFetcher(companyRepo, companyRelationRepo, aiClient, gbizInfoService)
	relationsFetcher.SetSearchBudget(companySearchBudget)
	relationsFetcher.SetSearchFlight(companySearchFlight)
	relationsFetcher.SetInfoFetcher(infoFetcher)
	jobFetcher := company.NewJobFetchService(companyRepo, aiClient)
	jobFetcher.SetSearchBudget(companySearchBudget)
	jobFetcher.SetSearchFlight(companySearchFlight)
	crawlService.SetInfoFetcher(infoFetcher)
	crawlService.SetJobFetcher(jobFetcher)
	auditLogService := admin.NewAuditLogService(auditLogRepo)
	authService.SetAuditLog(auditLogService)
	analysisService := analysis.NewAnalysisScoringService(
		userWeightScoreRepo,
		chatMessageRepo,
		progressRepo,
		conversationContextRepo,
		userEmbeddingRepo,
		jobEmbeddingRepo,
		matchRepo,
		aiClient,
		nil,
	)
	interviewService := interview.NewInterviewService(interviewSessionRepo, interviewUtteranceRepo, interviewReportRepo, userRepo, emailService, aiClient, realtimeUsageService)
	if jobEnqueuer != nil {
		interviewService.SetJobEnqueuer(jobEnqueuer)
	}
	interviewService.StartWorker()
	if rdb != nil {
		queueServer = queue.NewServer(rdb)
		queueServer.RegisterHandlers(emailService, interviewService)
		if err := queueServer.Start(); err != nil {
			log.Printf("[queue] failed to start worker: %v", err)
		}
	}

	// クロス機能連携サービス（チャットスコア↔面接/職務経歴書レビュー）
	crossFeatureService := flywheel.NewCrossFeatureIntegrationService(userWeightScoreRepo)
	interviewService.SetCrossFeatureService(crossFeatureService)
	interviewService.SetCompanyQuestionRepo(interviewCompanyQuestionRepo)
	interviewService.SetQuestionStateRepo(interviewQuestionStateRepo)
	interviewService.SetSkillScoreRepo(skillScoreRepo)
	interviewService.SetCompanyRepo(companyRepo)
	resumeService.SetCrossFeatureService(crossFeatureService)

	// コントローラー層の初期化
	organizationRepo := repositories.NewOrganizationRepository(db)
	organizationService := organization.NewOrganizationService(organizationRepo)
	authController := controllers.NewAuthController(authService)
	oauthController := controllers.NewOAuthController(oauthService, organizationService)
	chatController := controllers.NewChatController(chatService, matchingService, analysisService, userRepo, emailService)
	questionController := controllers.NewQuestionController(questionService)
	relationController := controllers.NewCompanyRelationController(companyQueryRepo, aiClient)
	companyValidator := company.NewCompanyValidationService(companyRepo, aiClient)
	companyValidator.SetSearchBudget(companySearchBudget)
	companyValidator.SetSearchFlight(companySearchFlight)
	relationController.SetCompanyValidator(companyValidator)
	resumeService.SetCompanyValidator(companyValidator)
	resumeService.SetCompanyRepo(companyRepo)
	adminCompanyController := controllers.NewAdminCompanyController(companyRepo, auditLogService, gbizInfoService, aiClient)
	adminCompanyController.SetCompanySearchGuards(companySearchBudget, companySearchFlight)
	adminCompanyController.SetRelationsFetcher(relationsFetcher)
	adminCrawlController := controllers.NewAdminCrawlController(crawlService, auditLogService)
	adminJobController := controllers.NewAdminJobController(companyRepo, jobCategoryRepo, graduateRepo, auditLogService)
	adminAuditController := controllers.NewAdminAuditController(auditLogService)
	// gBizINFO 公式 API を使った企業データ収集パイプライン
	// Mynavi・Rikunabi・CareerTasu スクレイパーは利用規約違反リスクのため削除 (#178)
	gbizToken := os.Getenv("GBIZINFO_API_TOKEN")
	companyGraphPipeline := &scraper.Pipeline{
		GBiz:      scraper.NewGBizClient("", gbizToken),
		Threshold: config.CompanyGraphThreshold(),
	}
	adminCompanyGraphController := controllers.NewAdminCompanyGraphController(companyGraphPipeline, companyRepo, companyRelationRepo, auditLogService, aiClient)
	adminCompanyGraphController.SetRelationsFetcher(relationsFetcher)
	resumeController := controllers.NewResumeController(resumeService)

	// S3 upload service for interview videos (optional — skipped if env vars are not set)
	s3UploadService, s3Err := storage.NewS3UploadService()
	if s3Err != nil {
		slog.Warn("S3 upload service not available", "error", s3Err)
		s3UploadService = nil
	}
	var objectDeleter auth.ObjectDeleter
	if s3UploadService != nil {
		objectDeleter = s3UploadService
		authService.SetObjectDeleter(s3UploadService)
	}
	userDeletionService := auth.NewUserDeletionService(db, objectDeleter, auditLogService)
	adminOrganizationController := controllers.NewAdminOrganizationController(organizationService)
	adminSchoolController := controllers.NewAdminSchoolController(schoolService)
	adminUserController := controllers.NewAdminUserController(userRepo, auditLogService)
	adminUserController.SetDeletionService(userDeletionService)
	adminUserController.SetSchoolService(schoolService)
	interviewController := controllers.NewInterviewController(interviewService, videoRepo, s3UploadService)
	realtimeController := controllers.NewRealtimeController(interviewService, realtimeUsageService)
	adminInterviewController := controllers.NewAdminInterviewController(interviewService, videoRepo, s3UploadService)
	adminInterviewController.SetCompanyQuestionRepo(interviewCompanyQuestionRepo)
	adminInterviewController.SetCompanyRepo(companyRepo)
	adminInterviewController.SetOpenAIClient(aiClient)
	adminInterviewController.SetUserAccessGuard(userDeletionService)
	adminInterviewController.SetSchoolAccess(userRepo, interviewSessionRepo, schoolService)
	adminDashboardController := controllers.NewAdminDashboardController(userRepo, interviewSessionRepo, interviewReportRepo)
	adminDashboardController.SetSchoolService(schoolService)
	adminCostsController := controllers.NewAdminCostsController(apiCostService, realtimeUsageService, companySearchBudget)
	adminVectorController := controllers.NewAdminVectorController(admin.NewAdminVectorService())
	profileRecalcService := flywheel.NewProfileRecalculationService(profileRecalcRepo, companyRepo)
	profileRecalcController := controllers.NewAdminProfileRecalculationController(profileRecalcService)
	companyEntryService := services.NewCompanyEntryService(db, userRepo, pendingRegistrationRepo, emailService)
	authService.SetCompanyOwnershipClaimer(companyEntryService)
	companyEntryController := controllers.NewCompanyEntryController(companyEntryService)
	releaseNoteService := services.NewReleaseNoteService(db, aiClient)
	releaseNoteController := controllers.NewReleaseNoteController(releaseNoteService, userRepo)
	// Discord経由での本番「指定日終日起動」登録(#829台のインフラ方針参照)。
	// AWS認証情報が取れない環境(ローカル等)でも起動は継続し、機能のみ無効化する。
	discordUptimeService, discordErr := discord.NewUptimeServiceFromEnv(context.Background())
	if discordErr != nil {
		log.Printf("[Discord] uptime service disabled: %v", discordErr)
		discordUptimeService = nil
	}
	discordInteractionController := controllers.NewDiscordInteractionController(discordUptimeService)
	githubController := controllers.NewGitHubController(githubService, skillScoreService)
	esRewriteController := controllers.NewESRewriteController(aiClient)
	scheduleRepo := repositories.NewScheduleRepository(db)
	scheduleService := schedule.NewScheduleService(scheduleRepo)
	// Googleカレンダー連携
	googleTokenRepo := repositories.NewUserGoogleTokenRepository(db)
	calendarSyncService := schedule.NewCalendarSyncService(googleTokenRepo, scheduleRepo, oauthConfig)
	scheduleService.SetCalendarSyncService(calendarSyncService)
	googleCalendarController := controllers.NewGoogleCalendarController(calendarSyncService)
	scheduleController := controllers.NewScheduleController(scheduleService)
	esReviewController := controllers.NewESReviewController()
	appService := application.NewApplicationService(appStatusRepo, matchRepo)
	appController := controllers.NewApplicationController(appService)
	integratedProfileController := controllers.NewIntegratedProfileController(crossFeatureService, interviewSessionRepo, resumeRepo)
	scoreValidationRepo := repositories.NewScoreValidationRepository(db)
	scoreValidationService := admin.NewScoreValidationService(scoreValidationRepo)
	scoreValidationController := controllers.NewAdminScoreValidationController(scoreValidationService)
	collectiveInsightRepo := repositories.NewCollectiveInsightRepository(db)
	collectiveInsightService := flywheel.NewCollectiveInsightService(collectiveInsightRepo, userWeightScoreRepo)
	collectiveInsightController := controllers.NewCollectiveInsightController(collectiveInsightService)
	scraperSessionService := admin.NewScraperSessionService(scraperSessionRepo)
	scraperSessionController := controllers.NewAdminScraperSessionController(scraperSessionService)

	// Echo初期化
	e := echo.New()
	e.HideBanner = true
	// カスタムエラーハンドラーとバリデーターを設定
	e.HTTPErrorHandler = middleware.CustomHTTPErrorHandler
	e.Validator = middleware.NewCustomValidator()

	// グローバルミドルウェア
	e.Use(echo.WrapMiddleware(middleware.RequestIDMiddleware))
	e.Use(echo.WrapMiddleware(middleware.RequestLoggerMiddleware))
	e.Use(echo.WrapMiddleware(securityHeadersMiddleware))
	e.Use(echo.WrapMiddleware(buildCORSMiddleware()))
	e.Use(routes.EchoTenantResolver(organizationService))

	// ヘルスチェックエンドポイント
	// /healthz は ECS ターゲットグループ・ALB・Kubernetes の標準パス
	// /health は後方互換のため維持
	healthHandler := func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
	e.GET("/health", healthHandler)
	e.GET("/healthz", healthHandler)

	// APIルートグループ
	api := e.Group("/api")

	// ルーティング設定
	routes.SetupAuthRoutes(api, authController, oauthController, cfg.UserSecret, userDeletionService, organizationService)
	routes.SetupChatRoutes(api, chatController, questionController, cfg.UserSecret, userDeletionService, organizationService)
	routes.SetupCompanyRoutes(api, relationController)
	routes.SetupAdminRoutes(api, adminCompanyController, adminCrawlController, adminJobController, adminUserController, adminOrganizationController, adminSchoolController, adminAuditController, adminCompanyGraphController, adminInterviewController, adminDashboardController, adminCostsController, profileRecalcController, scoreValidationController, collectiveInsightController, scraperSessionController, adminVectorController, userRepo, schoolService, cfg.AdminSecret)
	routes.SetupResumeRoutes(api, resumeController, cfg.UserSecret, userDeletionService, organizationService)
	routes.SetupInterviewRoutes(api, interviewController, realtimeController, cfg.UserSecret, userDeletionService, organizationService)
	routes.SetupGitHubRoutes(api, githubController, cfg.UserSecret, userDeletionService, organizationService)
	routes.SetupESRoutes(api, esRewriteController, esReviewController)
	routes.SetupScheduleRoutes(api, scheduleController, cfg.UserSecret, userDeletionService, organizationService)
	routes.SetupGoogleCalendarRoutes(api, googleCalendarController, cfg.UserSecret, userDeletionService, organizationService)
	routes.SetupApplicationRoutes(api, appController, cfg.UserSecret, userDeletionService, organizationService)
	routes.SetupUserRoutes(api, integratedProfileController)
	routes.SetupCollectiveInsightRoutes(api, collectiveInsightController, cfg.UserSecret, userDeletionService, organizationService)
	api.POST("/company-entry", companyEntryController.Submit, echoCompanyEntryRateLimit())
	api.GET("/whats-new", releaseNoteController.List, routes.EchoUserAuth(cfg.UserSecret, userDeletionService))
	adminEntry := api.Group("/admin", routes.EchoAdminAuth(userRepo, cfg.AdminSecret))
	adminEntry.POST("/company-entry-submissions/:id/resend-email", companyEntryController.ResendEmail)
	// CI(GitHub Actions)からのマシン間呼び出しのため、ログインユーザー前提のEchoAdminAuthではなく
	// 共有シークレットのみで認証する(#861)
	api.POST("/admin/whats-new/ingest", releaseNoteController.Ingest, routes.EchoStaticSecretAuth(cfg.AdminSecret))
	// Discord Interactions Endpoint。認証はEd25519署名検証(DISCORD_PUBLIC_KEY)で行うため
	// 通常のuser/adminミドルウェアは使わない。
	api.POST("/discord/interactions", discordInteractionController.Interactions)

	go crawlService.StartScheduler()

	// 退会ユーザーの猶予期間経過後の物理削除（1日1回）
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		run := func() {
			n, err := userDeletionService.PurgeExpiredWithdrawals(time.Now().UTC())
			if err != nil {
				slog.Error("purge expired withdrawals failed", "error", err)
				return
			}
			if n > 0 {
				slog.Info("purged expired withdrawals", "count", n)
			}
		}
		run()
		for range ticker.C {
			run()
		}
	}()

	// サーバー起動
	port := cfg.ServerPort
	slog.Info("Starting server", "port", port)
	if err := e.Start(":" + port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func echoCompanyEntryRateLimit() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := middleware.GetClientIP(c.Request())
			if !middleware.CompanyEntryRateLimiter.Allow(ip) {
				return echo.NewHTTPError(http.StatusTooManyRequests, "Too Many Requests: 投稿回数の上限に達しました。しばらく待ってから再試行してください。")
			}
			return next(c)
		}
	}
}
