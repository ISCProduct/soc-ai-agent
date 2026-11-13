package chat

import (
	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/controllers/httpapi"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/chat"
	"Backend/internal/services/costs"
	"Backend/internal/services/diagnosis"
	"Backend/internal/services/email"
	ifaces "Backend/internal/services/interfaces"
	"Backend/internal/services/matching"
	"Backend/internal/services/shared"
	"Backend/internal/usagectx"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

type ChatController struct {
	chatService     ifaces.ChatService
	matchingService ifaces.MatchingService
	analysisService ifaces.AnalysisScoringService
	userRepo        repository.UserRepository
	emailService    ifaces.EmailService
	jobs            shared.JobEnqueuer
	qualityRepo     *repositories.DiagnosisQualityRepository
	matchingTimers  sync.Map // key: sessionID, value: *time.Timer（デバウンス用）
	matchingRunning sync.Map // key: sessionID, value: struct{}（実行中の多重起動防止, #1167）
	aiBudget        costs.AIBudgetStatusProvider
}

const matchingRetryMaxAttempts = 3
const matchingDebounceDelay = 3 * time.Second

// matchingRunTimeout はバックグラウンドのマッチング計算1回分(リトライ込み)の上限（#1167）。
// #1061 で最悪所要が20分規模まで縮んだため、その1.5倍を打ち切り点にする。
// これを超えるのは入力規模か外部APIの異常であり、待ち続けるより goroutine と
// DB接続を解放した方がよい。本番は Fargate タスク1本のため滞留がそのまま他の
// リクエストのコネクション枯渇になる。
//
// ponytail: 打ち切りが実際に効くのは LLM 呼び出し(ctx を渡している)とリトライ待ちの間。
// リポジトリ層は ctx を受け取らないため、実行中のDBクエリ1本ぶんは超過しうる。
// そこまで詰めるならリポジトリの signature に ctx を通す必要がある。
const matchingRunTimeout = 30 * time.Minute

func NewChatController(chatService ifaces.ChatService, matchingService ifaces.MatchingService, analysisService ifaces.AnalysisScoringService, userRepo repository.UserRepository, emailService ifaces.EmailService) *ChatController {
	return &ChatController{
		chatService:     chatService,
		matchingService: matchingService,
		analysisService: analysisService,
		userRepo:        userRepo,
		emailService:    emailService,
	}
}

// SetAIBudget はバックグラウンドのマッチングで AI 理由を作る前の予算判定を設定する（#1776）。
// 設定しないと、上限を超えた利用者にも AI 理由の費用がかかる。
func (c *ChatController) SetAIBudget(p costs.AIBudgetStatusProvider) {
	c.aiBudget = p
}

// SetJobEnqueuer は診断妥当性ジョブ等の投入先を設定する。
func (c *ChatController) SetJobEnqueuer(j shared.JobEnqueuer) {
	c.jobs = j
}

// SetDiagnosisQualityRepo は recommendations に診断信頼度を載せるための参照先。
func (c *ChatController) SetDiagnosisQualityRepo(r *repositories.DiagnosisQualityRepository) {
	c.qualityRepo = r
}

func (c *ChatController) loadDiagnosisQuality(userID uint, sessionID string) (confidence int, flags []string, summary string) {
	if c.qualityRepo == nil {
		return 0, nil, ""
	}
	report, err := c.qualityRepo.FindByUserAndSession(userID, sessionID)
	if err != nil || report == nil {
		return 0, nil, ""
	}
	return report.Confidence, diagnosis.ParseFlagsJSON(report.FlagsJSON), report.Summary
}

func parseEmailListFromEnv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		addr := strings.TrimSpace(p)
		if addr != "" {
			out = append(out, addr)
		}
	}
	return out
}

func (c *ChatController) notifyMatchingFailure(userID uint, sessionID string, err error) {
	if c.emailService == nil {
		return
	}
	recipients := parseEmailListFromEnv("MATCHING_ALERT_EMAILS")
	if len(recipients) == 0 {
		return
	}
	subject := fmt.Sprintf("[SOC AI] Background matching failed (user=%d)", userID)
	body := fmt.Sprintf(
		"Background matching permanently failed after retries.\nuser_id=%d\nsession_id=%s\nerror=%v\n",
		userID,
		sessionID,
		err,
	)
	if sendErr := c.emailService.SendSystemAlertEmail(recipients, subject, body); sendErr != nil {
		log.Printf("[Chat] matching failure alert email failed: %v", sendErr)
	}
}

func (c *ChatController) runBackgroundMatching(userID uint, sessionID string) {
	// 同一セッションの計算が走っている間は捨てる（#1167）。
	// デバウンスは「まだ発火していないタイマー」しか止められないため、計算が数分かかると
	// 前の計算中に次が始まり、同じ user_company_matches 行への UPDATE が競合する。
	//
	// ponytail: 捨てた分の最新メッセージは、次のメッセージでの再スケジュールまで反映されない。
	// 反映漏れが問題になるなら、実行中フラグに「保留」を持たせて完了後に1回だけ再実行する
	// (coalescing) へ広げる。
	if _, running := c.matchingRunning.LoadOrStore(sessionID, struct{}{}); running {
		log.Printf("[Chat] Background matching already running, skipped: user=%d session=%s", userID, sessionID)
		return
	}
	defer c.matchingRunning.Delete(sessionID)

	// バックグラウンドには actor が届かない。載せないと AI 理由の費用が誰の予算にも入らない。
	// 予算判定もリクエストの context にしか載らないので、ここで載せ直す。
	ctx, cancel := context.WithTimeout(usagectx.WithActor(context.Background(), userID, 0), matchingRunTimeout)
	defer cancel()
	ctx = costs.WithUserAIBudgetCheck(ctx, c.aiBudget, userID)

	var lastErr error

retryLoop:
	for attempt := 1; attempt <= matchingRetryMaxAttempts; attempt++ {
		err := c.matchingService.CalculateMatching(ctx, userID, sessionID)
		if err == nil {
			log.Printf("[Chat] Background matching calculation completed: user=%d session=%s attempt=%d", userID, sessionID, attempt)
			c.enqueueDiagnosisQuality(userID, sessionID)
			return
		}
		lastErr = err

		// 打ち切り後のリトライは同じ ctx で必ず失敗するため、ここで止める。
		if ctx.Err() != nil {
			log.Printf("[Chat] Background matching aborted: user=%d session=%s attempt=%d err=%v reason=%v", userID, sessionID, attempt, err, ctx.Err())
			break
		}

		if attempt == matchingRetryMaxAttempts {
			break
		}

		wait := time.Duration(1<<(attempt-1)) * time.Second
		log.Printf("[Chat] Background matching calculation failed: user=%d session=%s attempt=%d/%d err=%v retry_in=%s", userID, sessionID, attempt, matchingRetryMaxAttempts, err, wait)
		// time.Sleep だと打ち切りを待てない。バックオフ中もキャンセルに応じる。
		select {
		case <-ctx.Done():
			// 打ち切りも「結果が出ていない」ことに変わりはないので、下のアラート経路へ合流する。
			log.Printf("[Chat] Background matching aborted during backoff: user=%d session=%s reason=%v", userID, sessionID, ctx.Err())
			break retryLoop
		case <-time.After(wait):
		}
	}

	log.Printf("[Chat] ALERT: background matching permanently failed: user=%d session=%s attempts=%d err=%v", userID, sessionID, matchingRetryMaxAttempts, lastErr)
	c.notifyMatchingFailure(userID, sessionID, lastErr)
}

func (c *ChatController) enqueueDiagnosisQuality(userID uint, sessionID string) {
	if c.jobs == nil {
		log.Printf("[Chat] diagnosis quality skipped (no queue): user=%d session=%s", userID, sessionID)
		return
	}
	if err := c.jobs.EnqueueDiagnosisQuality(userID, sessionID); err != nil {
		log.Printf("[Chat] diagnosis quality enqueue failed: user=%d session=%s err=%v", userID, sessionID, err)
	}
}

// scheduleBackgroundMatching はセッション単位でデバウンスしてマッチング計算をスケジュールする。
// 同一セッションへの連続メッセージが来ても、最後のメッセージから matchingDebounceDelay 後に1回だけ実行する。
func (c *ChatController) scheduleBackgroundMatching(userID uint, sessionID string) {
	// 既存タイマーがあればキャンセル
	if prev, ok := c.matchingTimers.Load(sessionID); ok {
		prev.(*time.Timer).Stop()
	}
	timer := time.AfterFunc(matchingDebounceDelay, func() {
		c.matchingTimers.Delete(sessionID)
		c.runBackgroundMatching(userID, sessionID)
	})
	c.matchingTimers.Store(sessionID, timer)
}

// checkSessionOwnership は session_id が既存セッションの場合、その所有者かを検証する。
// メッセージが1件も無い新規セッションは誰でも開始できるため許可する。
//
// 以前は session_id だけで全件読んで先頭1件の UserID を比較していたため、
// 「1セッションのメッセージは全て同一ユーザー」という不変条件に依存していた（#1156）。
// その不変条件は DB 制約でもクエリでも担保されていない。
//
// 「自分のメッセージが1件でもあれば許可」では不十分で、他人のメッセージが混在した
// セッションで双方が通ってしまう。session_id はクライアント採番（Math.random ベース）
// なので衝突・先回り書き込みがあり得る。よって他人の存在を常に確認し、
// 1件でもあれば双方に対して拒否する（フェイルクローズ）。
func (c *ChatController) ensureSessionNotOwnedByOthers(sessionID string, userID uint) error {
	hasOther, err := c.chatService.SessionHasOtherUserMessages(sessionID, userID)
	if err != nil {
		return err
	}
	if hasOther {
		return shared.ErrForbidden
	}
	// 他人がいない = 新規セッション or 自分のセッション。どちらも許可
	return nil
}

// checkSessionOwnership は所有者を検証したうえで自分の履歴を返す。
// 履歴が要らない呼び出し側は ensureSessionNotOwnedByOthers を使う
// （content TEXT を含む全件 SELECT を発行しないため）。
func (c *ChatController) checkSessionOwnership(sessionID string, userID uint) ([]models.ChatMessage, error) {
	if err := c.ensureSessionNotOwnedByOthers(sessionID, userID); err != nil {
		return nil, err
	}
	return c.chatService.GetChatHistoryForUser(sessionID, userID)
}

// Chat チャット処理
func (c *ChatController) Chat(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	var req chat.ChatRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// user_id は認証ヘッダーから設定（リクエストボディの値は無視）
	req.UserID = userID

	if req.SessionID == "" || req.Message == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Missing required fields")
	}

	// 既存セッションへの書き込みの場合、他ユーザーのセッションでないことを検証する（#946 IDOR対策）。
	// さらに NEW セッションの同時開始時には session_id の所有権を原子的に確定させ、
	// TOCTOU を防ぐ（#963）。この判定は他人のメッセージの有無によるフェイルクローズと同時に行う。
	if err := c.chatService.ClaimSessionOwnership(req.SessionID, userID); err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, "Forbidden")
		}
		return httpapi.InternalError(err)
	}
	if err := c.ensureSessionNotOwnedByOthers(req.SessionID, userID); err != nil {
		if err == shared.ErrForbidden {
			return echo.NewHTTPError(http.StatusForbidden, "Forbidden")
		}
		return httpapi.InternalError(err)
	}

	resp, err := c.chatService.ProcessChat(ctx.Request().Context(), req)
	if err != nil {
		return httpapi.InternalError(err)
	}

	// マッチング計算をデバウンスして非同期実行（同一セッションへの連続リクエストを1回にまとめる）
	c.scheduleBackgroundMatching(req.UserID, req.SessionID)

	return ctx.JSON(http.StatusOK, resp)
}

// GetHistory チャット履歴取得
func (c *ChatController) GetHistory(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	sessionID := ctx.QueryParam("session_id")
	if sessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "session_id is required")
	}

	history, err := c.checkSessionOwnership(sessionID, userID)
	if err != nil {
		if err == shared.ErrForbidden {
			return echo.NewHTTPError(http.StatusForbidden, "Forbidden")
		}
		return httpapi.InternalError(err)
	}

	return ctx.JSON(http.StatusOK, history)
}

// GetScores ユーザースコア取得
func (c *ChatController) GetScores(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	sessionID := ctx.QueryParam("session_id")
	if sessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "session_id is required")
	}

	scores, err := c.chatService.GetUserScores(userID, sessionID)
	if err != nil {
		return httpapi.InternalError(err)
	}

	return ctx.JSON(http.StatusOK, scores)
}

// GetRecommendations トップ適性企業を取得
func (c *ChatController) GetRecommendations(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	sessionID := ctx.QueryParam("session_id")
	if sessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "session_id is required")
	}

	limit := httpapi.LimitQuery(ctx, "limit", 10)

	// 既存のマッチング結果を取得（事前計算済みを想定）
	log.Printf("[GetRecommendations] Fetching pre-calculated matches for user %d, session %s\n", userID, sessionID)
	matches, err := c.matchingService.GetTopMatches(ctx.Request().Context(), userID, sessionID, limit)
	log.Printf("[GetRecommendations] Retrieved %d matches in fast mode\n", len(matches))

	if err != nil || len(matches) == 0 {
		log.Printf("[GetRecommendations] No matching results found, returning empty result\n")
		diagnostics, diagErr := c.matchingService.GetDiagnostics(userID, sessionID)
		if diagErr != nil {
			log.Printf("[GetRecommendations] Diagnostics error: %v\n", diagErr)
		}

		userScores, scoreErr := c.chatService.GetUserScores(userID, sessionID)
		if scoreErr != nil {
			log.Printf("[GetRecommendations] Failed to load user scores for provisional status: %v\n", scoreErr)
			userScores = []entity.UserWeightScore{}
		}

		evaluatedCategories := matching.CountEvaluatedCategories(userScores)
		diagConf, diagFlags, diagSummary := c.loadDiagnosisQuality(userID, sessionID)
		return ctx.JSON(http.StatusOK, matching.RecommendationResponse{
			Recommendations:     []matching.Recommendation{},
			Reason:              matching.EmptyRecommendationReason(diagnostics),
			Diagnostics:         diagnostics,
			EvaluatedCategories: evaluatedCategories,
			IsProvisional:       matching.IsRecommendationProvisional(evaluatedCategories, diagConf, diagFlags, 0),
			DiagnosisConfidence: diagConf,
			DiagnosisFlags:      diagFlags,
			DiagnosisSummary:    diagSummary,
		})
	}

	userScores, err := c.chatService.GetUserScores(userID, sessionID)
	if err != nil {
		log.Printf("[GetRecommendations] Failed to load user scores: %v\n", err)
		userScores = []entity.UserWeightScore{}
	}
	evaluatedCategories := matching.CountEvaluatedCategories(userScores)
	// 理由文はマッチ度と同じ合流後の適性で書く（評価済みカテゴリ数はチャット診断の進み具合なので合流しない）
	reasonScores := c.matchingService.ReasonScores(userID, userScores)
	diagConf, diagFlags, diagSummary := c.loadDiagnosisQuality(userID, sessionID)

	displayed := matching.DisplayableMatches(matches)
	minAxes := matching.MinMatchedAxisCount(displayed)
	return ctx.JSON(http.StatusOK, matching.RecommendationResponse{
		Recommendations:     matching.BuildRecommendations(displayed, reasonScores),
		EvaluatedCategories: evaluatedCategories,
		IsProvisional:       matching.IsRecommendationProvisional(evaluatedCategories, diagConf, diagFlags, minAxes),
		DiagnosisConfidence: diagConf,
		DiagnosisFlags:      diagFlags,
		DiagnosisSummary:    diagSummary,
		MinMatchedAxisCount: minAxes,
	})
}

// ToggleFavorite お気に入りをトグル (POST /api/chat/favorite)
func (c *ChatController) ToggleFavorite(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	var req struct {
		MatchID uint `json:"match_id"`
	}
	if err := ctx.Bind(&req); err != nil || req.MatchID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "match_id is required")
	}

	if err := c.matchingService.ToggleFavorite(req.MatchID, userID); err != nil {
		if err == shared.ErrForbidden {
			return echo.NewHTTPError(http.StatusForbidden, "Forbidden")
		}
		if err == shared.ErrNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "match not found")
		}
		return httpapi.InternalError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]bool{"ok": true})
}

// GetAnalysisSummary 4分析スコアと進捗を取得
func (c *ChatController) GetAnalysisSummary(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	sessionID := ctx.QueryParam("session_id")
	if sessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "session_id is required")
	}

	if c.analysisService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "analysis service not available")
	}

	summary, err := c.analysisService.BuildAnalysisSummary(ctx.Request().Context(), userID, sessionID)
	if err != nil {
		return httpapi.InternalError(err)
	}

	return ctx.JSON(http.StatusOK, summary)
}

// SendReport チャット分析結果をメールで送信
func (c *ChatController) SendReport(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if req.SessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "session_id is required")
	}

	// ユーザー情報取得
	user, err := c.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return echo.NewHTTPError(http.StatusNotFound, "User not found")
	}

	// ゲストユーザーは送信不可
	if user.IsGuest {
		return echo.NewHTTPError(http.StatusForbidden, "Guest users cannot receive email reports")
	}

	// 分析サマリー取得
	summary, err := c.analysisService.BuildAnalysisSummary(ctx.Request().Context(), userID, req.SessionID)
	if err != nil {
		return httpapi.InternalError(err)
	}

	// おすすめ企業取得（最大5件）
	matches, _ := c.matchingService.GetTopMatches(ctx.Request().Context(), userID, req.SessionID, 5)
	userScores, _ := c.chatService.GetUserScores(userID, req.SessionID)
	userScores = c.matchingService.ReasonScores(userID, userScores)

	var companies []email.EmailReportCompany
	for i, match := range matches {
		if match.Company.ID == 0 {
			continue
		}
		companies = append(companies, email.EmailReportCompany{
			Rank:   i + 1,
			Name:   match.Company.Name,
			Score:  int(match.MatchScore),
			Reason: matching.BuildMatchReason(match, userScores),
		})
	}

	// メール送信
	if err := c.emailService.SendAnalysisReport(user, summary, companies, req.SessionID); err != nil {
		log.Printf("[SendReport] Failed to send email: %v\n", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to send email")
	}

	return ctx.JSON(http.StatusOK, map[string]string{
		"message": "分析レポートを " + user.Email + " に送信しました",
	})
}

// GetSessions ユーザーのチャットセッション一覧を取得
func (c *ChatController) GetSessions(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	sessions, err := c.chatService.GetUserChatSessions(userID)
	if err != nil {
		return httpapi.InternalError(err)
	}

	return ctx.JSON(http.StatusOK, sessions)
}
