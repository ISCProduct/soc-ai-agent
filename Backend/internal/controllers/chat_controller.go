package controllers

import (
	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/services"
	ifaces "Backend/internal/services/interfaces"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
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
	matchingTimers  sync.Map // key: sessionID, value: *time.Timer（デバウンス用）
}

const minEvaluatedCategoriesForFinal = 4
const matchingRetryMaxAttempts = 3
const matchingDebounceDelay = 3 * time.Second

func NewChatController(chatService ifaces.ChatService, matchingService ifaces.MatchingService, analysisService ifaces.AnalysisScoringService, userRepo repository.UserRepository, emailService ifaces.EmailService) *ChatController {
	return &ChatController{
		chatService:     chatService,
		matchingService: matchingService,
		analysisService: analysisService,
		userRepo:        userRepo,
		emailService:    emailService,
	}
}

func countEvaluatedCategories(scores []entity.UserWeightScore) int {
	count := 0
	for _, score := range scores {
		if score.Score != 0 {
			count++
		}
	}
	return count
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
	ctx := context.Background()
	var lastErr error

	for attempt := 1; attempt <= matchingRetryMaxAttempts; attempt++ {
		err := c.matchingService.CalculateMatching(ctx, userID, sessionID)
		if err == nil {
			log.Printf("[Chat] Background matching calculation completed: user=%d session=%s attempt=%d", userID, sessionID, attempt)
			return
		}
		lastErr = err

		if attempt == matchingRetryMaxAttempts {
			break
		}

		wait := time.Duration(1<<(attempt-1)) * time.Second
		log.Printf("[Chat] Background matching calculation failed: user=%d session=%s attempt=%d/%d err=%v retry_in=%s", userID, sessionID, attempt, matchingRetryMaxAttempts, err, wait)
		time.Sleep(wait)
	}

	log.Printf("[Chat] ALERT: background matching permanently failed: user=%d session=%s attempts=%d err=%v", userID, sessionID, matchingRetryMaxAttempts, lastErr)
	c.notifyMatchingFailure(userID, sessionID, lastErr)
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

// Chat チャット処理
func (c *ChatController) Chat(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	var req services.ChatRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// user_id は認証ヘッダーから設定（リクエストボディの値は無視）
	req.UserID = userID

	if req.SessionID == "" || req.Message == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Missing required fields")
	}

	resp, err := c.chatService.ProcessChat(ctx.Request().Context(), req)
	if err != nil {
		return echoInternalError(err)
	}

	// マッチング計算をデバウンスして非同期実行（同一セッションへの連続リクエストを1回にまとめる）
	c.scheduleBackgroundMatching(req.UserID, req.SessionID)

	return ctx.JSON(http.StatusOK, resp)
}

// GetHistory チャット履歴取得
func (c *ChatController) GetHistory(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	sessionID := ctx.QueryParam("session_id")
	if sessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "session_id is required")
	}

	history, err := c.chatService.GetChatHistory(sessionID)
	if err != nil {
		return echoInternalError(err)
	}

	// セッション所有者チェック：最初のメッセージの UserID と照合
	if len(history) > 0 && history[0].UserID != 0 && history[0].UserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "Forbidden")
	}

	return ctx.JSON(http.StatusOK, history)
}

// GetScores ユーザースコア取得
func (c *ChatController) GetScores(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	sessionID := ctx.QueryParam("session_id")
	if sessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "session_id is required")
	}

	scores, err := c.chatService.GetUserScores(userID, sessionID)
	if err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, scores)
}

// GetRecommendations トップ適性企業を取得
func (c *ChatController) GetRecommendations(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	sessionID := ctx.QueryParam("session_id")
	limitStr := ctx.QueryParam("limit")

	if sessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "session_id is required")
	}

	limit := 10 // デフォルト
	if limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err == nil && l > 0 {
			limit = l
		}
	}

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

		reason := "matching_results_empty"
		if diagnostics != nil {
			if diagnostics.UserScoreCount == 0 {
				reason = "insufficient_user_scores"
			} else if diagnostics.ActiveCompanyCount == 0 || diagnostics.WeightProfileCount == 0 {
				reason = "insufficient_company_data"
			}
		}

		type RecommendationResponse struct {
			Recommendations     []any                 `json:"recommendations"`
			Reason              string                        `json:"reason,omitempty"`
			Diagnostics         *services.MatchingDiagnostics `json:"diagnostics,omitempty"`
			EvaluatedCategories int                           `json:"evaluated_categories"`
			IsProvisional       bool                          `json:"is_provisional"`
		}

		evaluatedCategories := countEvaluatedCategories(userScores)
		isProvisional := evaluatedCategories < minEvaluatedCategoriesForFinal
		response := RecommendationResponse{
			Recommendations:     []any{},
			Reason:              reason,
			Diagnostics:         diagnostics,
			EvaluatedCategories: evaluatedCategories,
			IsProvisional:       isProvisional,
		}

		return ctx.JSON(http.StatusOK, response)
	}

	// フロントエンド用のレスポンス形式に変換
	type CategoryScores struct {
		Technical     float64 `json:"technical"`
		Teamwork      float64 `json:"teamwork"`
		Leadership    float64 `json:"leadership"`
		Creativity    float64 `json:"creativity"`
		Stability     float64 `json:"stability"`
		Growth        float64 `json:"growth"`
		WorkLife      float64 `json:"work_life"`
		Challenge     float64 `json:"challenge"`
		Detail        float64 `json:"detail"`
		Communication float64 `json:"communication"`
	}

	type CompanyRecommendation struct {
		ID             int            `json:"id"`
		MatchID        uint           `json:"match_id"`      // マッチングレコードID（お気に入り操作に使用）
		CategoryName   string         `json:"category_name"` // 企業名
		Score          int            `json:"score"`         // マッチスコア
		Reason         string         `json:"reason"`        // マッチ理由
		Industry       string         `json:"industry"`
		Location       string         `json:"location"`
		Employees      string         `json:"employees"`
		TechStack      []string       `json:"tech_stack"`
		CategoryScores CategoryScores `json:"category_scores"`
		IsFavorited    bool           `json:"is_favorited"`
		IsApplied      bool           `json:"is_applied"`
	}

	type RecommendationResponse struct {
		Recommendations     []CompanyRecommendation `json:"recommendations"`
		EvaluatedCategories int                     `json:"evaluated_categories"`
		IsProvisional       bool                    `json:"is_provisional"`
	}

	userScores, err := c.chatService.GetUserScores(userID, sessionID)
	if err != nil {
		log.Printf("[GetRecommendations] Failed to load user scores: %v\n", err)
		userScores = []entity.UserWeightScore{}
	}
	evaluatedCategories := countEvaluatedCategories(userScores)
	isProvisional := evaluatedCategories < minEvaluatedCategoriesForFinal

	var items []CompanyRecommendation
	for _, match := range matches {
		if match.Company == nil || match.Company.ID == 0 {
			continue
		}

		employeeCount := "未定"
		if match.Company.EmployeeCount > 0 {
			employeeCount = strconv.Itoa(match.Company.EmployeeCount) + "名"
		}

		techStack := []string{}
		if match.Company.TechStack != "" {
			// 簡易的にカンマ区切りで分割
			techStack = splitTechStack(match.Company.TechStack)
		}

		items = append(items, CompanyRecommendation{
			ID:           int(match.Company.ID),
			MatchID:      match.ID,
			CategoryName: match.Company.Name,
			Score:        int(match.MatchScore),
			Reason:       services.BuildMatchReason(match, userScores),
			Industry:     match.Company.Industry,
			Location:     match.Company.Location,
			Employees:    employeeCount,
			TechStack:    techStack,
			IsFavorited:  match.IsFavorited,
			IsApplied:    match.IsApplied,
			CategoryScores: CategoryScores{
				Technical:     match.TechnicalMatch,
				Teamwork:      match.TeamworkMatch,
				Leadership:    match.LeadershipMatch,
				Creativity:    match.CreativityMatch,
				Stability:     match.StabilityMatch,
				Growth:        match.GrowthMatch,
				WorkLife:      match.WorkLifeMatch,
				Challenge:     match.ChallengeMatch,
				Detail:        match.DetailMatch,
				Communication: match.CommunicationMatch,
			},
		})
	}

	response := RecommendationResponse{
		Recommendations:     items,
		EvaluatedCategories: evaluatedCategories,
		IsProvisional:       isProvisional,
	}

	return ctx.JSON(http.StatusOK, response)
}

// ToggleFavorite お気に入りをトグル (POST /api/chat/favorite)
func (c *ChatController) ToggleFavorite(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
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
		if err == services.ErrForbidden {
			return echo.NewHTTPError(http.StatusForbidden, "Forbidden")
		}
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]bool{"ok": true})
}

// GetAnalysisSummary 4分析スコアと進捗を取得
func (c *ChatController) GetAnalysisSummary(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
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
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, summary)
}

// generateReasonForCategory カテゴリごとのマッチング理由を生成（フォールバック用）
func generateReasonForCategory(category string, _ int) string {
	reasons := map[string]string{
		"技術志向":        "最新技術への探求心と技術的な深掘りが評価されています。技術主導型の企業で活躍できるでしょう。",
		"コミュニケーション能力": "優れた対話力と説明力が認められています。チーム協業が重視される企業に適しています。",
		"リーダーシップ":     "主導性と意思決定力が強みです。マネジメント志向のキャリアパスが開かれています。",
		"チームワーク":      "協働と協調性に優れています。大規模チームでの開発に向いています。",
		"問題解決力":       "論理思考と分析力が際立っています。課題解決型のプロジェクトで力を発揮できます。",
		"創造性・発想力":     "独創性と革新的思考が光ります。スタートアップや新規事業で活躍できる素質があります。",
		"計画性・実行力":     "目標設定とタスク管理能力が高く評価されています。プロジェクト型企業に最適です。",
		"学習意欲・成長志向":   "継続学習と成長意識が強みです。教育体制が充実した企業で大きく成長できるでしょう。",
		"ストレス耐性・粘り強さ": "困難への対処力とプレッシャー対応力が優れています。高負荷環境でも安定したパフォーマンスを発揮できます。",
		"ビジネス思考・目標志向": "ビジネス価値の理解と成果志向が強みです。事業会社での活躍が期待されます。",
	}

	if reason, ok := reasons[category]; ok {
		return reason
	}
	return "あなたの特性が評価されています。この分野で活躍できる企業とマッチングしました。"
}

// generateMatchReason 企業マッチングの理由を生成
func generateMatchReason(match *entity.UserCompanyMatch) string {
	// AIで生成された理由があればそれを使用
	if match.MatchReason != "" {
		return match.MatchReason
	}

	// 各スコアを収集
	scores := map[string]float64{
		"技術力":        match.TechnicalMatch,
		"チームワーク":     match.TeamworkMatch,
		"リーダーシップ":    match.LeadershipMatch,
		"創造性・発想力":    match.CreativityMatch,
		"安定志向":       match.StabilityMatch,
		"成長意欲":       match.GrowthMatch,
		"ワークライフバランス": match.WorkLifeMatch,
		"挑戦意欲":       match.ChallengeMatch,
		"緻密さ":        match.DetailMatch,
		"コミュニケーション力": match.CommunicationMatch,
	}

	// トップ3を抽出
	type scoreItem struct {
		name  string
		score float64
	}
	var items []scoreItem
	for name, score := range scores {
		items = append(items, scoreItem{name, score})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})

	// 具体的な理由を生成
	if len(items) >= 3 {
		return fmt.Sprintf("総合マッチ度%.0f%% - あなたの%s(%.0f%%)、%s(%.0f%%)、%s(%.0f%%)が企業の求める人材像と高く一致しています。特に%sを活かせる環境で、即戦力として活躍が期待できます。",
			match.MatchScore,
			items[0].name, items[0].score,
			items[1].name, items[1].score,
			items[2].name, items[2].score,
			items[0].name)
	} else if len(items) >= 1 {
		return fmt.Sprintf("総合マッチ度%.0f%% - 特にあなたの%sが企業文化と合致しており、スムーズな適応が見込めます。",
			match.MatchScore, items[0].name)
	}

	return fmt.Sprintf("総合マッチ度%.0f%% - あなたの適性と企業の求める人材像が合致しています。", match.MatchScore)
}

// splitTechStack 技術スタック文字列を配列に変換
func splitTechStack(techStack string) []string {
	if techStack == "" {
		return []string{}
	}
	// JSONパース試行、失敗したらカンマ区切り
	var stack []string
	if err := json.Unmarshal([]byte(techStack), &stack); err == nil {
		return stack
	}
	// カンマ区切りにフォールバック
	parts := strings.Split(techStack, ",")
	result := []string{}
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// SendReport チャット分析結果をメールで送信
func (c *ChatController) SendReport(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
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
		return echoInternalError(err)
	}

	// おすすめ企業取得（最大5件）
	matches, _ := c.matchingService.GetTopMatches(ctx.Request().Context(), userID, req.SessionID, 5)
	userScores, _ := c.chatService.GetUserScores(userID, req.SessionID)

	var companies []services.EmailReportCompany
	for i, match := range matches {
		if match.Company == nil || match.Company.ID == 0 {
			continue
		}
		companies = append(companies, services.EmailReportCompany{
			Rank:   i + 1,
			Name:   match.Company.Name,
			Score:  int(match.MatchScore),
			Reason: services.BuildMatchReason(match, userScores),
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
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	sessions, err := c.chatService.GetUserChatSessions(userID)
	if err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, sessions)
}
