package main

import (
	"Backend/internal/config"
	abilitycontrollers "Backend/internal/controllers/ability"
	admincontrollers "Backend/internal/controllers/admin"
	applicationcontrollers "Backend/internal/controllers/application"
	assessmentcontrollers "Backend/internal/controllers/assessment"
	authcontrollers "Backend/internal/controllers/auth"
	chatcontrollers "Backend/internal/controllers/chat"
	companycontrollers "Backend/internal/controllers/company"
	escontrollers "Backend/internal/controllers/es"
	githubcontrollers "Backend/internal/controllers/github"
	insightcontrollers "Backend/internal/controllers/insight"
	interviewcontrollers "Backend/internal/controllers/interview"
	notificationcontrollers "Backend/internal/controllers/notification"
	releasecontrollers "Backend/internal/controllers/release"
	resumecontrollers "Backend/internal/controllers/resume"
	schedulecontrollers "Backend/internal/controllers/schedule"
	usercontrollers "Backend/internal/controllers/user"
	"Backend/internal/infrastructure/redisx"
	"Backend/internal/logger"
	"Backend/internal/middleware"
	"Backend/internal/observability"
	"Backend/internal/openai"
	"Backend/internal/queue"
	"Backend/internal/repositories"
	"Backend/internal/routes"
	"Backend/internal/scraper"
	abilitysvc "Backend/internal/services/ability"
	"Backend/internal/services/admin"
	"Backend/internal/services/analysis"
	"Backend/internal/services/application"
	assessmentsvc "Backend/internal/services/assessment"
	"Backend/internal/services/auth"
	"Backend/internal/services/chat"
	"Backend/internal/services/company"
	"Backend/internal/services/companyauth"
	"Backend/internal/services/companyportal"
	"Backend/internal/services/costs"
	"Backend/internal/services/diagnosis"
	"Backend/internal/services/email"
	"Backend/internal/services/esreview"
	"Backend/internal/services/flywheel"
	"Backend/internal/services/gbizinfo"
	"Backend/internal/services/github"
	"Backend/internal/services/houjinbangou"
	"Backend/internal/services/hr"
	"Backend/internal/services/interview"
	"Backend/internal/services/matching"
	notificationsvc "Backend/internal/services/notification"
	"Backend/internal/services/oauth"
	"Backend/internal/services/organization"
	"Backend/internal/services/refreshtoken"
	"Backend/internal/services/release"
	"Backend/internal/services/resume"
	"Backend/internal/services/schedule"
	"Backend/internal/services/school"
	"Backend/internal/services/schoolapproval"
	"Backend/internal/services/shared"
	"Backend/internal/services/skillscore"
	"Backend/internal/services/storage"
	"Backend/internal/services/teacher"
	"Backend/internal/services/training"
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
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, X-User-ID, X-User-Token, X-Admin-Email, X-Admin-Token, X-Company-User-ID, X-Company-User-Token")

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

	flushSentry, sentryOn := observability.InitSentry()
	defer flushSentry()

	cfg, db, aiClient, oauthConfig := bootstrap()

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
	// 学生・無認証向けの企業参照は審査前のゲスト投稿を隠す（#1203）
	companyPublicRepo := repositories.NewCompanyPublicRepository(db)
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
	// 管理者・企業所有者の判定(応募管理・学生分析・面接の企業ブリーフが使う)
	companyAccessRepo := repositories.NewCompanyAccessRepository(db)
	// APIコストモニタリング
	apiCallLogRepo := repositories.NewAPICallLogRepository(db)
	aiBudgetRepo := repositories.NewAIBudgetRepository(db)
	aiBudgetService := costs.NewAIBudgetService(aiBudgetRepo)
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
	aiClient.OnUsage = func(u openai.Usage) {
		apiCostService.LogUsage(u)
	}
	// ローカル推論先の障害時に OpenAI へ無条件でフォールバックすると、障害が続く間
	// 全トラフィックが従量課金へ移る。日次/月次のUSD上限と分間レートで打ち切る(#1293)
	aiClient.SetFallbackGuard(costs.NewOpenAIFallbackGuard(apiCallLogRepo))
	schoolRepo := repositories.NewSchoolRepository(db)
	schoolService := school.NewSchoolService(schoolRepo, userRepo)
	authService := auth.NewAuthService(userRepo, pendingRegistrationRepo, emailService)
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
	// web_search は検索結果が固定8,000トークン/callで課金される。info/relations/tech が
	// 別々に検索すると1社あたり3回分かかるため、検索結果を共有して1回に寄せる(#1124)。
	companySharedSearch := company.NewSearchContext()
	infoFetcher.SetSharedSearch(companySharedSearch)
	// 関連企業として新規作成された会社(gbizinfo経由/AI検索経由の両方)にも
	// infoFetcherで詳細情報を充填する(空データの企業が量産される問題への対応)。
	gbizInfoService.SetDetailFetcher(infoFetcher)
	// gBizINFO は法人番号が無いと何も引けない。商号しか分かっていない企業のために、
	// 国税庁 法人番号システムで法人番号を特定できるようにする(未設定なら無効)。
	gbizInfoService.SetCorporateNumberFinder(houjinbangou.NewClientFromEnv())
	relationsFetcher := company.NewCompanyRelationsFetcher(companyRepo, companyRelationRepo, aiClient, gbizInfoService)
	relationsFetcher.SetSearchBudget(companySearchBudget)
	relationsFetcher.SetSearchFlight(companySearchFlight)
	relationsFetcher.SetSharedSearch(companySharedSearch)
	relationsFetcher.SetInfoFetcher(infoFetcher)
	jobFetcher := company.NewJobFetchService(companyRepo, aiClient)
	jobFetcher.SetSearchBudget(companySearchBudget)
	jobFetcher.SetSearchFlight(companySearchFlight)
	crawlService.SetInfoFetcher(infoFetcher)
	crawlService.SetJobFetcher(jobFetcher)
	auditLogService := admin.NewAuditLogService(auditLogRepo)
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
	// 面接1回ごとに STT / LLM / TTS を叩くため、セッション数がそのまま費用に効く。
	// 既定は監視のみで、止めるのは INTERVIEW_BUDGET_ENFORCE を明示したときだけ。
	interviewService.SetBudgetGuard(costs.NewInterviewBudgetService(interviewSessionRepo, costs.NotifyInterviewBudgetToDiscord))
	if jobEnqueuer != nil {
		interviewService.SetJobEnqueuer(jobEnqueuer)
	}
	interviewService.StartWorker()

	diagnosisQualityRepo := repositories.NewDiagnosisQualityRepository(db)
	diagnosisQualityService := diagnosis.NewQualityService(
		diagnosisQualityRepo,
		userWeightScoreRepo,
		chatMessageRepo,
		matchRepo,
		aiClient,
	)

	if rdb != nil {
		queueServer = queue.NewServer(rdb)
		queueServer.RegisterHandlers(emailService, interviewService, diagnosisQualityService)
		if err := queueServer.Start(); err != nil {
			log.Printf("[queue] failed to start worker: %v", err)
		}
	}

	// クロス機能連携サービス（チャットスコア↔面接/職務経歴書レビュー）
	crossFeatureService := flywheel.NewCrossFeatureIntegrationService(userWeightScoreRepo)
	interviewService.SetCrossFeatureService(crossFeatureService)
	interviewService.SetMatchingRunner(matchingService)
	interviewService.SetAIBudget(aiBudgetService)
	interviewService.SetCompanyQuestionRepo(interviewCompanyQuestionRepo)
	interviewService.SetQuestionStateRepo(interviewQuestionStateRepo)
	interviewService.SetSkillScoreRepo(skillScoreRepo)
	interviewService.SetCompanyRepo(companyPublicRepo)
	interviewService.SetCompanyOwnerChecker(func(userID, companyID uint) (bool, error) {
		return shared.UserOwnsCompany(companyAccessRepo, userID, companyID)
	})
	resumeService.SetCrossFeatureService(crossFeatureService)

	// コントローラー層の初期化
	organizationRepo := repositories.NewOrganizationRepository(db)
	organizationService := organization.NewOrganizationService(organizationRepo)
	authController := authcontrollers.NewAuthController(authService, cfg.UserSecret)
	oauthController := authcontrollers.NewOAuthController(oauthService, organizationService)
	chatController := chatcontrollers.NewChatController(chatService, matchingService, analysisService, userRepo, emailService)
	chatController.SetAIBudget(aiBudgetService)
	if jobEnqueuer != nil {
		chatController.SetJobEnqueuer(jobEnqueuer)
	}
	chatController.SetDiagnosisQualityRepo(diagnosisQualityRepo)
	questionController := chatcontrollers.NewQuestionController(questionService)
	relationController := companycontrollers.NewCompanyRelationController(companyQueryRepo, aiClient)
	companyValidator := company.NewCompanyValidationService(companyPublicRepo, aiClient)
	companyValidator.SetSearchBudget(companySearchBudget)
	companyValidator.SetSearchFlight(companySearchFlight)
	relationController.SetCompanyValidator(companyValidator)
	resumeService.SetCompanyValidator(companyValidator)
	resumeService.SetCompanyRepo(companyPublicRepo)
	// 企業briefの「重視傾向」が未生成なら1回だけ作る（#1124）。
	// DB の企業情報だけを材料にした安いモデルの1コールで、結果は全学生で使い回される。
	resumeService.SetPersonaEnsurer(jobFetcher)
	// DB に無い企業は取得して登録する。取得結果は companies に保存され、
	// 企業検索・マッチング・面接ヒントからも参照できるようになる（#1124）。
	resumeService.SetCompanyProvisioner(infoFetcher)
	adminCompanyController := admincontrollers.NewAdminCompanyController(companyRepo, auditLogService, gbizInfoService, aiClient)
	adminCompanyController.SetCompanySearchGuards(companySearchBudget, companySearchFlight)
	adminCompanyController.SetSchoolRestrictionChecker(func(adminUserID uint) (bool, error) {
		restricted, _, err := schoolService.ResolveAdminAccess(adminUserID)
		return restricted, err
	})
	// コンストラクタが自前生成した infoFetcher には SetSharedSearch が掛からない。
	// 共有済みのものに差し替えないと、fetch-missing-batch で検索が統合されない(#1124)。
	adminCompanyController.SetInfoFetcher(infoFetcher)
	adminCompanyController.SetRelationsFetcher(relationsFetcher)
	adminCrawlController := admincontrollers.NewAdminCrawlController(crawlService, auditLogService)
	adminJobController := admincontrollers.NewAdminJobController(companyRepo, jobCategoryRepo, graduateRepo, auditLogService)
	adminAuditController := admincontrollers.NewAdminAuditController(auditLogService)
	// gBizINFO 公式 API を使った企業データ収集パイプライン
	// Mynavi・Rikunabi・CareerTasu スクレイパーは利用規約違反リスクのため削除 (#178)
	// 環境変数名の解決は config.LoadConfig に一本化する。
	// ここで os.Getenv("GBIZINFO_API_TOKEN") を直接読んでいたため、
	// GBIZINFO_API_KEY しか設定していない本番・staging では空文字が渡り、
	// 企業グラフの gBizINFO 取得だけが 401 になっていた。
	companyGraphPipeline := &scraper.Pipeline{
		GBiz:      scraper.NewGBizClient("", cfg.GBizInfoToken),
		Threshold: config.CompanyGraphThreshold(),
	}
	adminCompanyGraphController := admincontrollers.NewAdminCompanyGraphController(companyGraphPipeline, companyRepo, companyRelationRepo, auditLogService, aiClient)
	adminCompanyGraphController.SetRelationsFetcher(relationsFetcher)
	resumeController := resumecontrollers.NewResumeController(resumeService)

	// S3 upload service for interview videos (optional — skipped if env vars are not set)
	s3UploadService, s3Err := storage.NewS3UploadService()
	if s3Err != nil {
		slog.Warn("S3 upload service not available", "error", s3Err)
		s3UploadService = nil
	}
	var objectDeleter auth.ObjectDeleter
	if s3UploadService != nil {
		objectDeleter = s3UploadService
		logger.EnableS3ErrorArchive(s3UploadService)
	}
	userDeletionService := auth.NewUserDeletionService(repositories.NewUserWithdrawalRepository(db), objectDeleter, auditLogService)
	// 本人退会も、管理者による退会・日次のパージ・アクセス遮断と同じインスタンスを使う。
	authService.SetDeletionService(userDeletionService)
	adminOrganizationController := admincontrollers.NewAdminOrganizationController(organizationService)
	adminSchoolController := admincontrollers.NewAdminSchoolController(schoolService)
	// 掲載申請の審査キュー (#1507)。申請テーブルと承認リスト(school_company_approvals)を束ねる。
	adminSchoolAppController := admincontrollers.NewAdminSchoolApplicationController(
		schoolapproval.NewReviewService(repositories.NewSchoolCompanyApplicationRepository(db), schoolRepo),
		schoolService,
	)
	// 個別求人停止 (#1508)。承認済み企業の求人でも学校ごとに止められる。
	adminSchoolJobSuppressionController := admincontrollers.NewAdminSchoolJobSuppressionController(
		schoolapproval.NewSuppressionService(repositories.NewSchoolJobSuppressionRepository(db)),
		schoolService,
	)
	adminUserController := admincontrollers.NewAdminUserController(userRepo, auditLogService)
	adminUserController.SetDeletionService(userDeletionService)
	adminUserController.SetSchoolService(schoolService)
	interviewController := interviewcontrollers.NewInterviewController(interviewService, videoRepo, s3UploadService)
	realtimeController := interviewcontrollers.NewRealtimeController(interviewService, realtimeUsageService)
	adminInterviewController := admincontrollers.NewAdminInterviewController(interviewService, videoRepo, s3UploadService)
	adminInterviewController.SetCompanyQuestionRepo(interviewCompanyQuestionRepo)
	adminInterviewController.SetCompanyRepo(companyRepo)
	adminInterviewController.SetOpenAIClient(aiClient)
	adminInterviewController.SetUserAccessGuard(userDeletionService)
	adminInterviewController.SetSchoolAccess(userRepo, interviewSessionRepo, schoolService)
	adminJobController.SetSchoolAccess(schoolService)
	adminDashboardController := admincontrollers.NewAdminDashboardController(userRepo, interviewSessionRepo, interviewReportRepo)
	adminDashboardController.SetSchoolService(schoolService)
	adminDashboardController.SetOrganizationService(organizationService)
	adminDashboardController.SetFunnelSource(repositories.NewConversionFunnelRepository(db))
	adminCostsController := admincontrollers.NewAdminCostsController(apiCostService, realtimeUsageService, companySearchBudget)
	adminAIBudgetController := admincontrollers.NewAIBudgetController(aiBudgetService, auditLogService)
	userAIBudgetController := usercontrollers.NewAIBudgetController(aiBudgetService)
	adminVectorController := admincontrollers.NewAdminVectorController(admin.NewAdminVectorService())
	profileRecalcService := flywheel.NewProfileRecalculationService(profileRecalcRepo, companyRepo)
	profileRecalcController := admincontrollers.NewAdminProfileRecalculationController(profileRecalcService)
	userScoreRecalculationService := admin.NewUserScoreRecalculationService(
		userWeightScoreRepo,
		repositories.NewUserScoreRecalculationRepository(db),
		chatService,
		matchingService,
	)
	userScoreRecalculationController := admincontrollers.NewUserScoreRecalculationController(userScoreRecalculationService)
	companyEntryService := company.NewCompanyEntryService(db, userRepo, pendingRegistrationRepo, emailService)
	authService.SetCompanyOwnershipClaimer(companyEntryService)
	companyEntryController := companycontrollers.NewCompanyEntryController(companyEntryService)
	companyUserRepo := repositories.NewCompanyUserRepository(db)
	companyUserRefreshRepo := repositories.NewCompanyUserRefreshTokenRepository(db)
	companyUserService := companyauth.NewCompanyUserService(db, companyUserRepo, companyUserRefreshRepo, emailService, cfg.CompanyUserSecret)
	companyAuthController := companycontrollers.NewCompanyAuthController(companyUserService)
	companyPortalController := companycontrollers.NewCompanyPortalController(companyUserService)
	adminCompanyUserController := companycontrollers.NewAdminCompanyUserController(companyUserService)
	releaseNoteService := release.NewReleaseNoteService(repositories.NewReleaseNoteRepository(db), aiClient)
	releaseNoteController := releasecontrollers.NewReleaseNoteController(releaseNoteService, userRepo)
	githubController := githubcontrollers.NewGitHubController(githubService, skillScoreService)
	esRewriteController := escontrollers.NewESRewriteController(aiClient)
	scheduleRepo := repositories.NewScheduleRepository(db)
	scheduleService := schedule.NewScheduleService(scheduleRepo)
	// Googleカレンダー連携
	googleTokenRepo := repositories.NewUserGoogleTokenRepository(db)
	calendarSyncService := schedule.NewCalendarSyncService(googleTokenRepo, scheduleRepo, oauthConfig)
	scheduleService.SetCalendarSyncService(calendarSyncService)
	googleCalendarController := schedulecontrollers.NewGoogleCalendarController(calendarSyncService)
	scheduleController := schedulecontrollers.NewScheduleController(scheduleService)
	esReviewController := escontrollers.NewESReviewController(aiClient)
	// ES添削の結果を履歴として残し、前回との差分とスコア連携を行う(#1531)。
	esReviewRepo := repositories.NewESReviewRepository(db)
	esReviewController.SetReviewService(esreview.NewService(esReviewRepo, crossFeatureService))
	// アプリ内通知（#2033）。応募・スカウト・先生の案内・予定の前日に、学生へ知らせる
	notificationService := notificationsvc.NewService(repositories.NewNotificationRepository(db))
	appService := application.NewApplicationService(appStatusRepo, matchRepo, db, companyAccessRepo)
	appService.SetNotifier(notificationService)
	appController := applicationcontrollers.NewApplicationController(appService)
	appController.SetSchoolAccess(schoolService)
	hrStudentAnalysisService := hr.NewStudentAnalysisService(
		companyAccessRepo,
		userRepo,
		crossFeatureService,
		userWeightScoreRepo,
		conversationContextRepo,
		interviewSessionRepo,
		interviewReportRepo,
		resumeRepo,
	)
	hrStudentAnalysisController := applicationcontrollers.NewHRStudentAnalysisController(hrStudentAnalysisService)
	// 企業ポータルの学生検索・タグ管理 (#1094)
	studentSearchRepo := repositories.NewStudentSearchRepository(db)
	companyStudentTagRepo := repositories.NewCompanyStudentTagRepository(db)
	studentSemanticClient := hr.NewStudentSemanticClient()
	hrStudentAnalysisService.SetIdentityRevealer(studentSearchRepo)
	studentSearchService := hr.NewStudentSearchService(studentSearchRepo, companyStudentTagRepo, studentSemanticClient)
	industryRepo := repositories.NewIndustryRepository(db)
	companyStudentController := companycontrollers.NewCompanyStudentController(studentSearchService, hrStudentAnalysisService, industryRepo)
	userPreferenceRepo := repositories.NewUserPreferenceRepository(db)
	// 希望条件の保存とプロフィール更新の両方から同じ同期処理を呼ぶ (#1094)
	scoutIndexSyncer := hr.NewStudentIndexSyncer(userPreferenceRepo, studentSemanticClient)
	authService.SetScoutIndexSyncer(scoutIndexSyncer)
	// 退会時のベクトル削除と、日次のリコンサイル(ReconcileScoutIndex)も同じ同期処理を使う（#1094, #1204）。
	userDeletionService.SetScoutIndexSyncer(scoutIndexSyncer)
	userPreferenceController := usercontrollers.NewUserPreferenceController(userPreferenceRepo, scoutIndexSyncer, industryRepo)
	integratedProfileController := usercontrollers.NewIntegratedProfileController(crossFeatureService, interviewSessionRepo, resumeRepo)
	entitlementController := usercontrollers.NewEntitlementController(organizationService)
	scoreValidationRepo := repositories.NewScoreValidationRepository(db)
	scoreValidationService := admin.NewScoreValidationService(scoreValidationRepo)
	scoreValidationController := admincontrollers.NewAdminScoreValidationController(scoreValidationService)
	diagnosisQualityController := admincontrollers.NewAdminDiagnosisQualityController(diagnosisQualityRepo)
	collectiveInsightRepo := repositories.NewCollectiveInsightRepository(db)
	collectiveInsightService := flywheel.NewCollectiveInsightService(collectiveInsightRepo, userWeightScoreRepo)
	collectiveInsightController := insightcontrollers.NewCollectiveInsightController(collectiveInsightService)
	scraperSessionService := admin.NewScraperSessionService(scraperSessionRepo)
	scraperSessionController := admincontrollers.NewAdminScraperSessionController(scraperSessionService)

	e := newEchoServer(sentryOn, organizationService)

	// APIルートグループ
	api := e.Group("/api")

	// ルーティング設定
	routes.SetupAuthRoutes(api, authController, oauthController, cfg.UserSecret, userDeletionService, organizationService)
	aiBudgetGuards := routes.NewAIBudgetGuards(aiBudgetService)
	routes.SetupChatRoutes(api, chatController, questionController, cfg.UserSecret, userDeletionService, organizationService, aiBudgetGuards)
	routes.SetupCompanyRoutes(api, relationController, cfg.UserSecret, userDeletionService, organizationService, aiBudgetGuards)
	industryWeightProfileRepo := repositories.NewIndustryWeightProfileRepository(db)
	teacherInsightService := teacher.NewStudentInsightService(userRepo, userWeightScoreRepo, industryRepo, industryWeightProfileRepo)
	// 低マッチのまま進行中の応募を教員一覧に出す（#1028）
	teacherInsightService.SetLowMatchReader(appStatusRepo)
	// 履歴書要対応を教員一覧に出す（#1030）
	teacherInsightService.SetResumeFactReader(resumeRepo)
	// 最終活動日と未活動の絞り込みを教員一覧に出す（#1573）
	teacherInsightService.SetLastActivityReader(userRepo)
	teacherInsightController := insightcontrollers.NewTeacherStudentInsightController(teacherInsightService)
	guidanceRepo := repositories.NewTeacherStudentGuidanceRepository(db)
	guidanceService := teacher.NewGuidanceService(guidanceRepo, userRepo)
	guidanceService.SetNotifier(notificationService)
	teacherGuidanceController := insightcontrollers.NewTeacherGuidanceController(guidanceService, schoolService)
	teacherESReviewController := insightcontrollers.NewTeacherESReviewController(esReviewRepo)
	studentGuidanceController := usercontrollers.NewStudentGuidanceController(guidanceService)
	routes.SetupAdminRoutes(api, adminCompanyController, adminCrawlController, adminJobController, adminUserController, adminOrganizationController, adminSchoolController, adminSchoolAppController, adminSchoolJobSuppressionController, adminAuditController, adminCompanyGraphController, adminInterviewController, adminDashboardController, adminCostsController, profileRecalcController, userScoreRecalculationController, scoreValidationController, diagnosisQualityController, collectiveInsightController, scraperSessionController, adminVectorController, appController, teacherInsightController, teacherGuidanceController, teacherESReviewController, userRepo, schoolService, cfg.AdminSecret, adminAIBudgetController)
	routes.SetupResumeRoutes(api, resumeController, cfg.UserSecret, userDeletionService, organizationService, aiBudgetGuards)
	// 適性検査 第1段（#1678）。結果は source=assessment で user_weight_scores へ入り、マッチングへ合流する
	assessmentRepo := repositories.NewAssessmentRepository(db)
	matchingService.SetAssessmentScoreReader(assessmentRepo)
	assessmentService := assessmentsvc.NewService(assessmentRepo, matchingService, userWeightScoreRepo)
	routes.SetupAssessmentRoutes(api, assessmentcontrollers.NewController(assessmentService), cfg.UserSecret, userDeletionService, organizationService, aiBudgetGuards)
	// 適性検査 第2段（#1675）。学校独自テンプレートはマッチングに合流させない
	routes.SetupAssessmentSchoolRoutes(api, assessmentcontrollers.NewSchoolController(assessmentsvc.NewSchoolService(assessmentRepo)), userRepo, schoolService, cfg.AdminSecret)

	// 適性検査 第3段: 能力検査（SOCAIAGENT-433）。言語・非言語の正答数を独立スコアとして持つ。
	// user_weight_scores へは書かない（10カテゴリへ変換する方法が無い）ので、
	// matchingService へは繋がない。性格検査とはテーブル・型・パスを分けている。
	abilityRepo := repositories.NewAbilityRepository(db)
	routes.SetupAbilityRoutes(api, abilitycontrollers.NewController(abilitysvc.NewService(abilityRepo)), cfg.UserSecret, userDeletionService, organizationService)
	routes.SetupInterviewRoutes(api, interviewController, realtimeController, cfg.UserSecret, userDeletionService, organizationService, aiBudgetGuards)
	routes.SetupGitHubRoutes(api, githubController, cfg.UserSecret, userDeletionService, organizationService, aiBudgetGuards)
	routes.SetupESRoutes(api, esRewriteController, esReviewController, cfg.UserSecret, userDeletionService, organizationService, aiBudgetGuards)
	routes.SetupScheduleRoutes(api, scheduleController, cfg.UserSecret, userDeletionService, organizationService)
	routes.SetupNotificationRoutes(api, notificationcontrollers.NewController(notificationService), cfg.UserSecret, userDeletionService, organizationService)
	routes.SetupGoogleCalendarRoutes(api, googleCalendarController, cfg.UserSecret, userDeletionService, organizationService)
	routes.SetupApplicationRoutes(api, appController, hrStudentAnalysisController, cfg.UserSecret, userDeletionService, organizationService)
	// 企業ポータルのダッシュボードと応募者管理 (#1320)。
	// 応募・求人・学生の集計はそれぞれ既存のリポジトリを使い、新しいテーブルは作らない。
	companyPortalApplicationController := companycontrollers.NewCompanyPortalApplicationController(
		appService, companyRepo, studentSearchRepo,
	)
	// 企業ポータルの求人管理 (#1321)。既存の CompanyRepository を使い、新しいテーブルは作らない。
	companyPortalJobController := companycontrollers.NewCompanyPortalJobController(
		companyportal.NewJobService(companyRepo),
	)
	companyPortalInterviewQuestionController := companycontrollers.NewCompanyPortalInterviewQuestionController(
		companyportal.NewInterviewQuestionService(interviewCompanyQuestionRepo),
	)
	// 自社プロフィール編集と担当者管理 (#1322)。
	// 担当者管理は既存の CompanyUserService をそのまま使う。
	companyPortalProfileController := companycontrollers.NewCompanyPortalProfileController(
		companyportal.NewProfileService(companyRepo), companyUserService,
	)
	// 学校への掲載申請 (#1506)。専用テーブル school_company_applications を使う。
	companyPortalSchoolApplicationController := companycontrollers.NewCompanyPortalSchoolApplicationController(
		companyportal.NewSchoolApplicationService(repositories.NewSchoolCompanyApplicationRepository(db)),
	)
	// スカウト送信・テンプレート (#1095)
	scoutRepo := repositories.NewScoutRepository(db)
	// 通知メールのリンクはデプロイが設定する APP_URL を使う。
	// FRONTEND_URL は staging / production に無く、未設定だと localhost になる。
	scoutService := companyportal.NewScoutService(scoutRepo, studentSearchRepo, userRepo, companyRepo, emailService, config.AppURL())
	scoutService.SetNotifier(notificationService)
	companyPortalScoutController := companycontrollers.NewCompanyPortalScoutController(scoutService)
	studentScoutController := usercontrollers.NewStudentScoutController(scoutService)
	routes.SetupCompanyAuthRoutes(api, companyAuthController, companyPortalController, companyPortalInterviewQuestionController, companyStudentController, companyPortalApplicationController, companyPortalJobController, companyPortalProfileController, companyPortalSchoolApplicationController, companyPortalScoutController, cfg.CompanyUserSecret, companyUserRepo, companyRepo)
	routes.SetupUserRoutes(api, integratedProfileController, entitlementController, userPreferenceController, studentGuidanceController, studentScoutController, cfg.UserSecret, userDeletionService, organizationService, userAIBudgetController)
	routes.SetupCollectiveInsightRoutes(api, collectiveInsightController, cfg.UserSecret, userDeletionService, organizationService)
	api.POST("/company-entry", companyEntryController.Submit, echoCompanyEntryRateLimit())
	api.GET("/whats-new", releaseNoteController.List, routes.EchoUserAuth(cfg.UserSecret, userDeletionService))
	// これらは企業ユーザー招待・学習データのエクスポート等、担当校に閉じない
	// 横断操作なので is_admin 必須にする（職員=staff は不可）。SetupAdminRoutes の外で
	// 作られているため EchoRequireAdmin をグループに明示的に掛ける(#職員ロール)。
	adminEntry := api.Group("/admin", routes.EchoAdminAuth(userRepo, cfg.AdminSecret), routes.EchoRequireAdmin())
	adminEntry.POST("/company-entry-submissions/:id/resend-email", companyEntryController.ResendEmail)
	adminEntry.POST("/companies/:id/company-users", adminCompanyUserController.Invite)
	adminEntry.GET("/companies/:id/company-users", adminCompanyUserController.List)
	adminEntry.PATCH("/companies/:id/company-users/:userID", adminCompanyUserController.SetDisabled)
	// 学習データのエクスポート(#268)。候補者の発話と選考結果を含むため管理者のみ。
	adminTrainingController := admincontrollers.NewAdminTrainingController(training.NewService(repositories.NewTrainingExportRepository(db)))
	// stats は件数しか返さない(個人情報を含まない)。運用から機械的に叩けるよう、
	// whats-new/ingest と同じサービス間認証にする。管理者になりすます形を避ける。
	api.GET("/admin/training/stats", adminTrainingController.Stats, routes.EchoStaticSecretAuth(cfg.ServiceSecret))
	// export は候補者の発話と選考結果を返すので、管理者本人の認証を要求する。
	adminEntry.GET("/training/export", adminTrainingController.Export)
	// CI(GitHub Actions)からのマシン間呼び出しのため、ログインユーザー前提のEchoAdminAuthではなく
	// 共有シークレットのみで認証する(#861)
	api.POST("/admin/whats-new/ingest", releaseNoteController.Ingest, routes.EchoStaticSecretAuth(cfg.ServiceSecret))

	startMaintenanceJobs(crawlService, userDeletionService, notificationService, pendingRegistrationRepo, refreshTokenRepo, companyUserRefreshRepo)

	startServer(e, cfg.ServerPort)
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
