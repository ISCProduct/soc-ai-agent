package main

import (
	"Backend/internal/config"
	"Backend/internal/controllers"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/repositories"
	"Backend/internal/routes"
	"Backend/internal/scraper"
	"Backend/internal/services"
	"log"
	"net/http"
	"os"
	"strings"
)

func buildAllowedOrigins() map[string]struct{} {
	allowedOrigins := make(map[string]struct{})
	raw := os.Getenv("ALLOWED_ORIGINS")

	for _, origin := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			continue
		}
		allowedOrigins[trimmed] = struct{}{}
	}

	// フェイルセーフ: ALLOWED_ORIGINS 未設定時は全オリジン拒否（#327）
	// ローカル開発時は .env に ALLOWED_ORIGINS=http://localhost:3000 を明示設定してください。
	if len(allowedOrigins) == 0 {
		log.Println("WARNING: ALLOWED_ORIGINS が未設定のため、全クロスオリジンリクエストを拒否します。")
	}

	return allowedOrigins
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

func corsMiddleware(next http.Handler) http.Handler {
	allowedOrigins := buildAllowedOrigins()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		_, isAllowedOrigin := allowedOrigins[origin]

		w.Header().Add("Vary", "Origin")
		if isAllowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-ID, X-User-Token, X-Admin-Email, X-Admin-Token")

		if r.Method == "OPTIONS" {
			if origin != "" && !isAllowedOrigin {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// checkAnnotationFont はサーバー起動時に PDF アノテーション用フォントの存在を確認し、
// 設定に問題がある場合は警告ログを出力する。
// フォントが存在しない場合もサーバー起動は継続するが、PDF 注釈が劣化する旨を明示する。
func checkAnnotationFont() {
	fontPath := os.Getenv("ANNOTATION_FONT_PATH")
	if fontPath == "" {
		log.Println("WARNING: ANNOTATION_FONT_PATH が設定されていません。" +
			"フォールバックフォントを使用します（日本語注釈が正常に表示されない可能性があります）。" +
			"環境変数 ANNOTATION_FONT_PATH にフォントパスを設定してください。")
		return
	}
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		log.Printf("WARNING: ANNOTATION_FONT_PATH のフォントが見つかりません: %q\n"+
			"PDF注釈の日本語レビューページが生成されない場合があります。\n"+
			"Dockerfileで fonts-noto-cjk がインストールされているか確認してください。", fontPath)
		return
	}
	log.Printf("INFO: PDF アノテーションフォント確認済み: %q", fontPath)
}

func main() {
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

	// マイグレーション実行
	if err := models.AutoMigrate(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	log.Println("Database migration completed")

	// 初期データ投入
	if err := models.SeedData(db); err != nil {
		log.Fatalf("Failed to seed data: %v", err)
	}
	log.Println("Database seeding completed")

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
	emailService := services.NewEmailService()
	apiCostService := services.NewAPICostService(apiCallLogRepo)
	realtimeUsageService := services.NewRealtimeUsageService(realtimeUsageRepo, emailService)
	// OpenAI APIコール時にトークン使用量をロギング
	aiClient.OnUsage = func(model string, promptTokens, completionTokens int) {
		apiCostService.LogCall(model, promptTokens, completionTokens)
	}
	authService := services.NewAuthService(userRepo, pendingRegistrationRepo, emailService)
	authService.SetDB(db)
	skillScoreService := services.NewSkillScoreService(skillScoreRepo)
	githubService := services.NewGitHubService(githubRepo, skillScoreService, aiClient)
	oauthService := services.NewOAuthService(userRepo, oauthConfig, githubService)
	chatService := services.NewChatService(aiClient, questionWeightRepo, chatMessageRepo, userWeightScoreRepo, aiGeneratedQuestionRepo, predefinedQuestionRepo, jobCategoryRepo, userRepo, userEmbeddingRepo, jobEmbeddingRepo, phaseRepo, progressRepo, sessionValidationRepo, conversationContextRepo)
	questionService := services.NewQuestionGeneratorService(aiClient, questionWeightRepo)
	matchingService := services.NewMatchingService(userWeightScoreRepo, companyRepo, matchRepo, aiClient)
	resumeService := services.NewResumeService(resumeRepo, "storage/resumes", aiClient)
	crawlService := services.NewCrawlService(crawlRepo, companyRepo, popularityRepo, aiClient)
	auditLogService := services.NewAuditLogService(auditLogRepo)
	analysisService := services.NewAnalysisScoringService(
		userWeightScoreRepo,
		chatMessageRepo,
		progressRepo,
		conversationContextRepo,
		userEmbeddingRepo,
		jobEmbeddingRepo,
		matchRepo,
		nil,
	)
	interviewService := services.NewInterviewService(interviewSessionRepo, interviewUtteranceRepo, interviewReportRepo, userRepo, emailService, aiClient, realtimeUsageService)
	interviewService.StartWorker()

	// クロス機能連携サービス（チャットスコア↔面接/職務経歴書レビュー）
	crossFeatureService := services.NewCrossFeatureIntegrationService(userWeightScoreRepo)
	interviewService.SetCrossFeatureService(crossFeatureService)
	resumeService.SetCrossFeatureService(crossFeatureService)

	// コントローラー層の初期化
	authController := controllers.NewAuthController(authService)
	oauthController := controllers.NewOAuthController(oauthService)
	chatController := controllers.NewChatController(chatService, matchingService, analysisService, userRepo, emailService)
	questionController := controllers.NewQuestionController(questionService)
	relationController := controllers.NewCompanyRelationController(companyQueryRepo, aiClient)
	adminCompanyController := controllers.NewAdminCompanyController(companyRepo, auditLogService, nil, aiClient)
	adminCrawlController := controllers.NewAdminCrawlController(crawlService, auditLogService)
	adminJobController := controllers.NewAdminJobController(companyRepo, jobCategoryRepo, graduateRepo, auditLogService)
	adminUserController := controllers.NewAdminUserController(userRepo, auditLogService)
	adminAuditController := controllers.NewAdminAuditController(auditLogService)
	// gBizINFO 公式 API を使った企業データ収集パイプライン
	// Mynavi・Rikunabi・CareerTasu スクレイパーは利用規約違反リスクのため削除 (#178)
	gbizToken := os.Getenv("GBIZINFO_API_TOKEN")
	companyGraphPipeline := &scraper.Pipeline{
		GBiz:      scraper.NewGBizClient("", gbizToken),
		Threshold: config.CompanyGraphThreshold(),
	}
	adminCompanyGraphController := controllers.NewAdminCompanyGraphController(companyGraphPipeline, companyRepo, companyRelationRepo, auditLogService)
	resumeController := controllers.NewResumeController(resumeService)

	// S3 upload service for interview videos (optional — skipped if env vars are not set)
	s3UploadService, s3Err := services.NewS3UploadService()
	if s3Err != nil {
		log.Printf("S3 upload service not available: %v", s3Err)
		s3UploadService = nil
	}
	interviewController := controllers.NewInterviewController(interviewService, videoRepo, s3UploadService)
	realtimeController := controllers.NewRealtimeController(interviewService, realtimeUsageService)
	adminInterviewController := controllers.NewAdminInterviewController(interviewService, videoRepo, s3UploadService)
	adminDashboardController := controllers.NewAdminDashboardController(userRepo, interviewSessionRepo, interviewReportRepo)
	adminCostsController := controllers.NewAdminCostsController(apiCostService, realtimeUsageService)
	profileRecalcService := services.NewProfileRecalculationService(profileRecalcRepo, companyRepo)
	profileRecalcController := controllers.NewAdminProfileRecalculationController(profileRecalcService)
	companyEntryController := controllers.NewCompanyEntryController(companyRepo, graduateRepo)
	githubController := controllers.NewGitHubController(githubService, skillScoreService)
	esRewriteController := controllers.NewESRewriteController(aiClient)
	scheduleRepo := repositories.NewScheduleRepository(db)
	scheduleService := services.NewScheduleService(scheduleRepo)
	scheduleController := controllers.NewScheduleController(scheduleService)
	esReviewController := controllers.NewESReviewController()
	appService := services.NewApplicationService(appStatusRepo, matchRepo)
	appController := controllers.NewApplicationController(appService)
	integratedProfileController := controllers.NewIntegratedProfileController(crossFeatureService, interviewSessionRepo, resumeRepo)
	scoreValidationRepo := repositories.NewScoreValidationRepository(db)
	scoreValidationService := services.NewScoreValidationService(scoreValidationRepo)
	scoreValidationController := controllers.NewAdminScoreValidationController(scoreValidationService)
	collectiveInsightRepo := repositories.NewCollectiveInsightRepository(db)
	collectiveInsightService := services.NewCollectiveInsightService(collectiveInsightRepo, userWeightScoreRepo)
	collectiveInsightController := controllers.NewCollectiveInsightController(collectiveInsightService)
	scraperSessionService := services.NewScraperSessionService(scraperSessionRepo)
	scraperSessionController := controllers.NewAdminScraperSessionController(scraperSessionService)

	// ルーティング設定
	routes.SetupAuthRoutes(authController, oauthController, userRepo, cfg.UserSecret)
	routes.SetupChatRoutes(chatController, questionController)
	routes.SetupCompanyRoutes(relationController)
	routes.SetupAdminRoutes(adminCompanyController, adminCrawlController, adminJobController, adminUserController, adminAuditController, adminCompanyGraphController, adminInterviewController, adminDashboardController, adminCostsController, profileRecalcController, scoreValidationController, collectiveInsightController, scraperSessionController, userRepo, cfg.AdminSecret)
	routes.SetupResumeRoutes(resumeController, userRepo, cfg.UserSecret)
	routes.SetupInterviewRoutes(interviewController, realtimeController)
	routes.SetupGitHubRoutes(githubController)
	routes.SetupESRoutes(esRewriteController, esReviewController)
	routes.SetupScheduleRoutes(scheduleController)
	routes.SetupApplicationRoutes(appController)
	routes.SetupUserRoutes(integratedProfileController)
	routes.SetupCollectiveInsightRoutes(collectiveInsightController)
	http.HandleFunc("/api/company-entry", companyEntryController.Submit)

	go crawlService.StartScheduler()

	// ヘルスチェックエンドポイント
	// /healthz は ECS ターゲットグループ・ALB・Kubernetes の標準パス
	// /health は後方互換のため維持
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}
	http.HandleFunc(
		"/health", healthHandler)
	http.HandleFunc("/healthz", healthHandler)

	// サーバー起動
	port := cfg.ServerPort
	if port == "" {
		port = "80"
	}

	log.Printf("Starting server on port %s...", port)
	if err := http.ListenAndServe(":"+port, securityHeadersMiddleware(corsMiddleware(http.DefaultServeMux))); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
