package main

import (
	"Backend/internal/config"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/repositories"
	"Backend/internal/routes"
	"Backend/internal/safego"
	"Backend/internal/services/auth"
	"Backend/internal/services/company"
	"Backend/internal/services/maintenance"
	"Backend/internal/services/organization"
	"Backend/migrations"
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"gorm.io/gorm"
)

// bootstrap は設定・DB・マイグレーション・シード・AIクライアントを用意する。失敗したら終了する。
func bootstrap() (*config.Config, *gorm.DB, *openai.Client, *config.OAuthConfig) {
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

	// AI クライアント初期化（#1293）
	//
	// OPENAI_API_KEY が無くても起動する。AI を使う呼び出しだけが ErrAIUnavailable で
	// 縮退し、企業検索・企業一覧・マッチング・選考管理・求人・キャッシュ済み説明は
	// 従来どおり動作する。エラーになるのは設定自体が成立しない場合
	// （AI_TEXT_PROVIDER=local なのに AI_TEXT_BASE_URL が無い等）だけ。
	aiClient, err := openai.NewFromEnv("")
	if err != nil {
		log.Fatalf("Failed to initialize AI client: %v", err)
	}
	textProvider, embeddingProvider, audioProvider := aiClient.Providers()
	slog.Info("AI providers resolved",
		"text", textProvider, "text_base_url", aiClient.BaseURL(), "text_model", aiClient.DefaultModel,
		"embedding", embeddingProvider, "embedding_base_url", aiClient.EmbeddingBaseURL(),
		"audio", audioProvider, "audio_base_url", aiClient.AudioBaseURL(),
		"degraded", aiClient.Degraded())

	// OAuth設定読み込み
	oauthConfig := config.LoadOAuthConfig()

	return cfg, db, aiClient, oauthConfig
}

// newEchoServer はグローバルミドルウェア・メトリクス・ヘルスチェックを設定した Echo を返す。
func newEchoServer(sentryOn bool, organizationService *organization.OrganizationService) *echo.Echo {
	// Echo初期化
	e := echo.New()
	e.HideBanner = true
	// カスタムエラーハンドラーとバリデーターを設定
	e.HTTPErrorHandler = middleware.CustomHTTPErrorHandler
	e.Validator = middleware.NewCustomValidator()

	// グローバルミドルウェア
	e.Use(echo.WrapMiddleware(middleware.RequestIDMiddleware))
	// Recover は Sentry の有無に関わらず入れる。DSN 未設定のときだけ panic で
	// 接続が切れる（観測基盤の有無でアプリの外部挙動が変わる）のを避ける。
	//
	// 並び順は「Recover が外、sentryecho が内」。Echo は先に Use したものが外側なので、
	// 逆にすると Recover が先に panic を拾ってしまい、sentryecho の
	// recoverWithSentry（スタックトレース付きで送る唯一の経路）が発火しない。
	e.Use(echomw.Recover())
	if sentryOn {
		// Repanic=true で Sentry へ送ったあと再 panic させ、外側の Recover が 500 にする（#1185）
		e.Use(sentryecho.New(sentryecho.Options{Repanic: true}))
	}
	// ボディサイズの上限。これが無いと ParseMultipartForm が超過分を一時ファイルへ
	// 無制限に書き出すため、各ハンドラのサイズ検査へ到達する前にディスクを埋められる。
	// 面接動画だけは maxVideoSize(500MB) を通す必要があるので、routes.BodyLimitSkipper
	// で除外し、ルート側に個別の上限を置く。
	e.Use(echomw.BodyLimitWithConfig(echomw.BodyLimitConfig{
		Skipper: routes.BodyLimitSkipper,
		Limit:   "32M",
	}))
	e.Use(middleware.EchoRequestLogger)
	e.Use(echo.WrapMiddleware(securityHeadersMiddleware))
	e.Use(echo.WrapMiddleware(buildCORSMiddleware()))
	e.Use(routes.EchoTenantResolver(organizationService))

	// メトリクス（#1186）。METRICS_TOKEN が未設定なら計装も公開もしない。
	// 本番の backend ALB はインターネット直結なので、既定で開けたままにはしない。
	// 取得方法は docs/wiki/operations.md「メトリクスを見る」を参照。
	if metricsToken := strings.TrimSpace(os.Getenv("METRICS_TOKEN")); metricsToken != "" {
		e.Use(echoprometheus.NewMiddlewareWithConfig(echoprometheus.MiddlewareConfig{
			Subsystem: "backend",
			// 除外理由は routes.MetricsSkipper のコメントを参照（ヘルスチェックと
			// ルート未一致リクエストのラベル爆発を避ける）。
			Skipper: routes.MetricsSkipper,
		}))
		e.GET("/metrics", echoprometheus.NewHandler(), routes.EchoMetricsAuth(metricsToken))
		log.Println("[metrics] /metrics を有効化しました（Bearer 認証）")
	}

	// ヘルスチェックエンドポイント
	// /healthz は ECS ターゲットグループ・ALB・Kubernetes の標準パス
	// /health は後方互換のため維持
	healthHandler := func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
	e.GET("/health", healthHandler)
	e.GET("/healthz", healthHandler)

	return e
}

// startMaintenanceJobs は定期実行ジョブを safego 経由で起動する。
func startMaintenanceJobs(crawlService *company.CrawlService, userDeletionService *auth.UserDeletionService, pendingRegistrationRepo *repositories.PendingRegistrationRepository, refreshTokenRepo *repositories.UserRefreshTokenRepository, companyUserRefreshRepo *repositories.CompanyUserRefreshTokenRepository) {
	// 以下の定期実行は safego 経由で起動する。素の go だと 1 回の panic で
	// Backend プロセスが丸ごと落ち、全ユーザーが 502 になる(#1405, #1446)。
	// Every は各回を個別に recover するので、1 回落ちても次の回は走る。

	// L1 温存ジョブの作成はクロール本体と分けて起動する。
	// 同じ goroutine に入れると、ここが落ちたときにクロールごと止まる。
	safego.Go(crawlService.EnsureL1WarmCrawlSource)
	safego.Every(1*time.Minute, false, crawlService.RunDueSources)

	// 退会ユーザーの猶予期間経過後の物理削除（1日1回）
	safego.Every(24*time.Hour, true, func() {
		// 退会時のベクトル削除はRAG障害でも退会を失敗させないためエラーを飲む。
		// 取りこぼしをここで消し直す(#1204)。パージより先に実行して、
		// 30日待たずに個人データを消す。
		if attempted, failed, err := userDeletionService.ReconcileScoutIndex(context.Background()); err != nil {
			slog.Error("scout index reconcile failed", "error", err)
		} else if failed > 0 {
			// 残り続けると気づけないので、0件になるまで毎回出す
			slog.Warn("scout index reconcile incomplete", "attempted", attempted, "failed", failed)
		}

		n, err := userDeletionService.PurgeExpiredWithdrawals(time.Now().UTC())
		if err != nil {
			slog.Error("purge expired withdrawals failed", "error", err)
			return
		}
		if n > 0 {
			slog.Info("purged expired withdrawals", "count", n)
		}
	})

	// 期限切れの仮登録・リフレッシュトークン（学生・企業ユーザー）の物理削除（1日1回、#2153・#2184）。
	// 退会ユーザーの処理とは別の周期にする。あちらは RAG への HTTP を退会者ごとに待つので、
	// 障害時に何時間も塞がり、DB だけで済むこの削除まで止まってしまうため（#2179 のレビュー）。
	// 件数と失敗は Run の中でログに出す。
	expiredTokenPurger := maintenance.NewExpiredTokenPurger(pendingRegistrationRepo, refreshTokenRepo, companyUserRefreshRepo, slog.Default())
	safego.Every(24*time.Hour, true, func() {
		expiredTokenPurger.Run(time.Now().UTC())
	})
}

// startServer は HTTP サーバーを起動する。戻るのは異常終了時のみ。
func startServer(e *echo.Echo, port string) {
	// サーバー起動
	slog.Info("Starting server", "port", port)
	// ヘッダーの読み取りと keep-alive の放置に上限を置く。WriteTimeout は履歴書レビューの
	// SSE を途中で切るので設定しない。予算判定はロックを持たないので、書き込みの遅い
	// クライアントが他の利用者を止めることはない。
	e.Server.ReadHeaderTimeout = 10 * time.Second
	e.Server.IdleTimeout = 120 * time.Second
	if err := e.Start(":" + port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
