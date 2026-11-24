package main

import (
	"Backend/internal/config"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/queue"
	"Backend/internal/repositories"
	"Backend/internal/routes"
	"Backend/internal/safego"
	"Backend/internal/services/admin"
	"Backend/internal/services/auth"
	"Backend/internal/services/company"
	"Backend/internal/services/interview"
	"Backend/internal/services/maintenance"
	notificationsvc "Backend/internal/services/notification"
	"Backend/internal/services/organization"
	"Backend/internal/services/prompts"
	"Backend/internal/services/videoretention"
	"Backend/migrations"
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
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

	// プロンプト版の環境変数が本文のある版を指しているかを確かめる（SOCAIAGENT-661）。
	// 本文の無い版のまま動くと、使用量ログの版が実際の本文を指さなくなるので起動を止める。
	// デプロイ前の確認は cmd/promptcheck で同じ検証だけを実行する。
	if err := prompts.ValidateSettings(); err != nil {
		log.Fatalf("Invalid prompt version settings: %v", err)
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
func startMaintenanceJobs(crawlService *company.CrawlService, userDeletionService *auth.UserDeletionService, notificationService *notificationsvc.Service, pendingRegistrationRepo *repositories.PendingRegistrationRepository, refreshTokenRepo *repositories.UserRefreshTokenRepository, companyUserRefreshRepo *repositories.CompanyUserRefreshTokenRepository, scoreValidationService *admin.ScoreValidationService, videoRetention *videoretention.Purger) {
	// 以下の定期実行は safego 経由で起動する。素の go だと 1 回の panic で
	// Backend プロセスが丸ごと落ち、全ユーザーが 502 になる(#1405, #1446)。
	// Every は各回を個別に recover するので、1 回落ちても次の回は走る。

	// L1 温存ジョブの作成はクロール本体と分けて起動する。
	// 同じ goroutine に入れると、ここが落ちたときにクロールごと止まる。
	safego.Go(crawlService.EnsureL1WarmCrawlSource)
	safego.Every(1*time.Minute, false, crawlService.RunDueSources)
	// 予定（締切・面接など）の前日通知（#2033）。同じ予定には1件だけ（dedupe_key）
	safego.Every(1*time.Hour, true, notificationService.NotifyUpcomingSchedules)

	// 退会ユーザーの猶予期間経過後の物理削除（1日1回）
	safego.Every(24*time.Hour, true, func() {
		// 退会時のベクトル削除はRAG障害でも退会を失敗させないためエラーを飲む。
		// 取りこぼしをここで消し直す(#1204)。パージより先に実行して、
		// パージ期限を待たずに個人データを消す。
		if attempted, failed, err := userDeletionService.ReconcileScoutIndex(context.Background()); err != nil {
			slog.Error("scout index reconcile failed", "error", err)
		} else if failed > 0 {
			// 残り続けると気づけないので、0件になるまで毎回出す
			slog.Warn("scout index reconcile incomplete", "attempted", attempted, "failed", failed)
		}

		// 使われなくなったゲスト（リフレッシュトークンの最終発行から7日）を退会させる（SOCAIAGENT-689）。
		// 通常の退会と同じく12か月残し、保持期間を過ぎたらこの下のパージで物理削除される。
		if expired, err := userDeletionService.ExpireStaleGuests(context.Background(), time.Now().UTC()); err != nil {
			slog.Error("expire stale guests failed", "error", err)
		} else if expired > 0 {
			slog.Info("expired stale guests", "count", expired)
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
	// 学校の保持期間を過ぎた面接動画（SOCAIAGENT-671）。退会の削除とは別。
	// 公開から30日の猶予が終わるまでは何も消さない。
	if videoRetention != nil {
		safego.Every(24*time.Hour, true, func() {
			n, err := videoRetention.Purge(context.Background(), time.Now().UTC())
			if err != nil {
				slog.Error("interview video retention purge failed", "error", err)
				return
			}
			if n > 0 {
				slog.Info("purged expired interview videos", "count", n)
			}
		})
	}

	expiredTokenPurger := maintenance.NewExpiredTokenPurger(pendingRegistrationRepo, refreshTokenRepo, companyUserRefreshRepo, slog.Default())
	safego.Every(24*time.Hour, true, func() {
		expiredTokenPurger.Run(time.Now().UTC())
	})

	// #203 スコアキャリブレーション自動適用（1日1回）。
	// 確定選考（通過/不採用）と応募時点スナップショットが揃ったカテゴリだけ重みを更新し、
	// 次の CalculateMatching から軸重みとして効く。サンプル不足・変化なしは静かにスキップ。
	if scoreValidationService != nil {
		safego.Every(24*time.Hour, true, func() {
			res, err := scoreValidationService.AutoCalibrateIfDue(time.Now().UTC())
			if err != nil {
				slog.Error("auto score calibration failed", "error", err)
				return
			}
			if res != nil {
				slog.Info("auto score calibration applied", "version", res.Version, "categories", len(res.Weights), "message", res.Message) // logpolicy:ok キャリブレーション結果の文言で、個人情報を含まない
			}
		})
	}
}

// shutdownBudget は ECS の stopTimeout（指定が無ければ 30 秒）の内側に収める。
// HTTP の停止、フォールバックのレポート生成、asynq の停止を、この1つの期限で並行に行う。
const shutdownBudget = 25 * time.Second

// startServer は HTTP サーバーを起動し、SIGTERM / SIGINT で処理中の要求を待ってから止める。
// デプロイのたびに in-flight のリクエストと、キューに入った面接レポートが切れていた（SOCAIAGENT-514）。
func startServer(e *echo.Echo, port string, interviewService *interview.InterviewService, queueServer *queue.Server) {
	slog.Info("Starting server", "port", port)
	// ヘッダーの読み取りと keep-alive の放置に上限を置く。WriteTimeout は履歴書レビューの
	// SSE を途中で切るので設定しない。予算判定はロックを持たないので、書き込みの遅い
	// クライアントが他の利用者を止めることはない。
	e.Server.ReadHeaderTimeout = 10 * time.Second
	e.Server.IdleTimeout = 120 * time.Second

	errCh := make(chan error, 1)
	// 起動の成否を errCh へ返す。safego.Go だと panic を握り潰して、
	// こちらの select がシグナルもエラーも受け取れず止まったままになる。
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				errCh <- fmt.Errorf("listen panicked: %v", rec)
			}
		}()
		errCh <- e.Start(":" + port)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed: %v", err)
		}
	case sig := <-sigCh:
		// ドレイン中の2回目は、チャネルに溜めずプロセスの既定動作（終了）に戻す。
		signal.Stop(sigCh)
		slog.Info("shutdown signal received", "signal", sig.String())
		shutdownWithinBudget(e, interviewService, queueServer)
	}
}

// shutdownWithinBudget は HTTP・面接レポート・asynq を同時に止め、全体を shutdownBudget で打ち切る。
// 直列に待つと 15 秒 + レポート生成 + asynq が ECS の 30 秒を超えて SIGKILL される。
func shutdownWithinBudget(e *echo.Echo, interviewService *interview.InterviewService, queueServer *queue.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownBudget)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("http shutdown panic", "panic", rec)
			}
		}()
		if err := e.Shutdown(ctx); err != nil {
			slog.Error("http shutdown", "error", err)
		}
	}()
	go func() {
		defer wg.Done()
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("queue shutdown panic", "panic", rec)
			}
		}()
		if queueServer == nil {
			return
		}
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			defer func() {
				if rec := recover(); rec != nil {
					slog.Error("queue shutdown panic", "panic", rec)
				}
			}()
			queueServer.Shutdown()
		}()
		select {
		case <-finished:
		case <-ctx.Done():
			slog.Error("queue shutdown exceeded budget")
		}
	}()
	if interviewService != nil {
		interviewService.StopWorker(ctx)
	}
	wg.Wait()
}
