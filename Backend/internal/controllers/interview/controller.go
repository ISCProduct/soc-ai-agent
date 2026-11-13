package interview

import (
	"Backend/domain/repository"
	"Backend/internal/aifailover"
	"Backend/internal/controllers/httpapi"
	ifaces "Backend/internal/services/interfaces"
	interviewsvc "Backend/internal/services/interview"
	"Backend/internal/services/shared"
	"Backend/internal/services/storage"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/labstack/echo/v4"
)

type InterviewController struct {
	interviewService ifaces.InterviewService
	// videoUpload は動画保存の設定が揃っているときだけ非nil。nil なら UploadVideo は 503。
	videoUpload *interviewsvc.VideoUploadService
}

func NewInterviewController(interviewService ifaces.InterviewService, videoRepo repository.InterviewVideoRepository, s3Service *storage.S3UploadService) *InterviewController {
	c := &InterviewController{interviewService: interviewService}
	// nil のポインタをインターフェースに入れると非nilになるので、ここで判定してから渡す。
	if videoRepo != nil && s3Service != nil {
		c.videoUpload = interviewsvc.NewVideoUploadService(videoRepo, s3Service)
	}
	return c
}

type interviewCreateRequest struct {
	Language          string `json:"language"`
	InterviewerGender string `json:"interviewer_gender"`
}

type interviewUtteranceRequest struct {
	Role string `json:"role"`
	Text string `json:"text"`
	// ClientUtteranceID は発話ごとにクライアントが発行する一意ID(#1476)。
	// 保存の再試行で同じ発話が二重に入らないよう、サーバー側の一意制約で弾くために使う。
	ClientUtteranceID string `json:"client_utterance_id"`
}

// GetTrend GET /api/interviews/trend?limit=N
// 完了済みセッションのスコア時系列を返す。
func (c *InterviewController) GetTrend(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	limit := httpapi.LimitQuery(ctx, "limit", 20)
	points, err := c.interviewService.GetTrend(userID, limit)
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"points": points,
	})
}

// GetReport GET /api/interviews/:id/report
func (c *InterviewController) GetReport(ctx echo.Context) error {
	sessionID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	report, err := c.interviewService.GetReport(userID, sessionID)
	if err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		return httpapi.InternalError(err)
	}
	if report == nil {
		return echo.NewHTTPError(http.StatusNotFound, "report not yet available")
	}
	return ctx.JSON(http.StatusOK, report)
}

// RegenerateReport POST /api/interviews/:id/report/regenerate
//
// 未生成のレポートを作り直すためにジョブを再投入する(#1476)。
// レポート生成ジョブは asynq の再試行を使い切ったりキューが溢れたりすると失われ、
// FinishSession は終了済みセッションを再キューしないため、これまで回復手段が無かった。
// フロントのレポート再試行ボタン（ポーリングのタイムアウト/失敗後）から叩く。
func (c *InterviewController) RegenerateReport(ctx echo.Context) error {
	sessionID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	queued, err := c.interviewService.RegenerateReport(userID, sessionID)
	if err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		if errors.Is(err, interviewsvc.ErrSessionNotFinished) {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		// キューへ入れられなかったのに202を返すと、フロントは存在しないジョブを
		// 3分ポーリングして再びタイムアウトするだけになる(#1476)。
		if errors.Is(err, interviewsvc.ErrReportQueueNotAvailable) {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "レポート生成を受け付けられませんでした。時間をおいて再試行してください。")
		}
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusAccepted, map[string]bool{"queued": queued})
}

// GetPhraseSuggestions GET /api/interviews/:id/phrase-suggestions
func (c *InterviewController) GetPhraseSuggestions(ctx echo.Context) error {
	sessionID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	suggestions, err := c.interviewService.GetPhraseSuggestions(ctx.Request().Context(), userID, sessionID)
	if err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"suggestions": suggestions,
	})
}

// SendReport POST /api/interviews/:id/send-report
func (c *InterviewController) SendReport(ctx echo.Context) error {
	sessionID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	if err := c.interviewService.SendReportEmail(userID, sessionID); err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		if err.Error() == "user not found" || err.Error() == "report not found" {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		if err.Error() == "guest users cannot receive email reports" {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]string{"message": "レポートをメールで送信しました"})
}

// maxVideoSize は受け付ける動画の最大サイズ（500 MB）
// UploadVideo POST /api/interviews/:id/upload-video
func (c *InterviewController) UploadVideo(ctx echo.Context) error {
	sessionID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}

	if c.videoUpload == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "video upload service not configured")
	}

	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	// IDOR対策: セッションが自分のものかを、ファイル解析/S3アップロードの前に検証する
	if err := c.interviewService.EnsureSessionOwnership(userID, sessionID); err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, "forbidden")
		}
		if errors.Is(err, repository.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "session not found")
		}
		return httpapi.InternalError(err)
	}

	// メモリには最大 10 MB を確保し、それ以上は一時ファイルに書き出す
	r := ctx.Request()
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "リクエストの解析に失敗しました。ファイルが破損しているか、サイズが大きすぎます")
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "動画ファイルが見つかりません")
	}

	video, err := c.videoUpload.Start(r.Context(), sessionID, userID, header.Size, header.Header.Get("Content-Type"), file)
	if err != nil {
		if errors.Is(err, interviewsvc.ErrVideoTooLarge) {
			return echo.NewHTTPError(http.StatusRequestEntityTooLarge,
				fmt.Sprintf("動画ファイルが大きすぎます（上限 %d MB）。録画設定を下げて再度お試しください", interviewsvc.MaxVideoSize>>20))
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "動画レコードの作成に失敗しました")
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"video_id": video.ID,
		"status":   "uploading",
		"message":  "動画のアップロードを開始しました",
	})
}

// Turn POST /api/interviews/:id/turn
func (c *InterviewController) Turn(ctx echo.Context) error {
	sessionID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}

	// multipart から音声と履歴を取得
	r := ctx.Request()
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Failed to parse form")
	}
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	// フォームから各パラメータを取得
	companyName := r.FormValue("company_name")
	companyReading := r.FormValue("company_reading")
	companyReadingResolved := r.FormValue("company_reading_resolved") == "true"
	position := r.FormValue("position")
	companyInfo := r.FormValue("company_info")
	companyType := r.FormValue("company_type")
	companyID := uint(parseFormInt(r, "company_id", 0))
	questionHints, err := parseCompanyQuestionHints(r.FormValue("question_hints"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company question hints")
	}

	turnCount := parseFormInt(r, "turn_count", 0)
	remainingSeconds := parseFormInt(r, "remaining_seconds", 0)
	questionIndex := parseFormInt(r, "question_index", 0)
	totalQuestions := parseFormInt(r, "total_questions", 0)
	questionElapsedSeconds := parseFormInt(r, "question_elapsed_seconds", 0)
	questionDurationSeconds := parseFormInt(r, "question_duration_seconds", 0)

	audioFile, _, err := r.FormFile("audio")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "audio file required")
	}
	defer audioFile.Close()
	audioData, err := io.ReadAll(audioFile)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to read audio")
	}

	result, err := c.interviewService.Turn(
		interviewsvc.WithCompanyQuestionHints(r.Context(), questionHints),
		userID,
		sessionID,
		audioData,
		companyName,
		companyReading,
		companyReadingResolved,
		position,
		companyInfo,
		companyType,
		companyID,
		turnCount,
		remainingSeconds,
		questionIndex,
		totalQuestions,
		questionElapsedSeconds,
		questionDurationSeconds,
	)
	if err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		if errors.Is(err, shared.ErrSessionFinished) {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		return httpapi.InternalError(err)
	}

	return writeTurnMultipart(ctx, map[string]any{
		"user_text":                result.UserText,
		"ai_text":                  result.AIText,
		"utterance_save_failed":    result.UtteranceSaveFailed,
		"question_source":          result.QuestionSource,
		"question_category":        result.QuestionCategory,
		"is_deepening":             result.IsDeepening,
		"total_questions":          result.TotalQuestions,
		"resolved_company_id":      result.ResolvedCompanyID,
		"custom_questions_enabled": result.CustomQuestionsEnabled,
		"ai_fallback":              result.AIFallback,
		"company_reading":          result.CompanyReading,
		"company_reading_resolved": result.CompanyReadingResolved,
	}, result.Audio)
}

// StartTurn POST /api/interviews/:id/start-turn
func (c *InterviewController) StartTurn(ctx echo.Context) error {
	sessionID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}

	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	var req struct {
		CompanyName             string   `json:"company_name"`
		CompanyReading          string   `json:"company_reading"`
		Position                string   `json:"position"`
		CompanyInfo             string   `json:"company_info"`
		CompanyType             string   `json:"company_type"`
		CompanyID               uint     `json:"company_id"`
		QuestionHints           []string `json:"question_hints"`
		QuestionIndex           int      `json:"question_index"`
		TotalQuestions          int      `json:"total_questions"`
		QuestionElapsedSeconds  int      `json:"question_elapsed_seconds"`
		QuestionDurationSeconds int      `json:"question_duration_seconds"`
	}
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := validateCompanyQuestionHints(req.QuestionHints); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company question hints")
	}

	result, err := c.interviewService.StartTurn(
		interviewsvc.WithCompanyQuestionHints(ctx.Request().Context(), req.QuestionHints),
		userID,
		sessionID,
		req.CompanyName,
		req.CompanyReading,
		req.Position,
		req.CompanyInfo,
		req.CompanyType,
		req.CompanyID,
		req.QuestionIndex,
		req.TotalQuestions,
		req.QuestionElapsedSeconds,
		req.QuestionDurationSeconds,
	)
	if err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		if errors.Is(err, shared.ErrSessionFinished) {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		return httpapi.InternalError(err)
	}

	return writeTurnMultipart(ctx, map[string]any{
		"ai_text":                  result.AIText,
		"utterance_save_failed":    result.UtteranceSaveFailed,
		"question_source":          result.QuestionSource,
		"question_category":        result.QuestionCategory,
		"is_deepening":             result.IsDeepening,
		"total_questions":          result.TotalQuestions,
		"resolved_company_id":      result.ResolvedCompanyID,
		"custom_questions_enabled": result.CustomQuestionsEnabled,
		"ai_fallback":              result.AIFallback,
		"company_reading":          result.CompanyReading,
		"company_reading_resolved": result.CompanyReadingResolved,
	}, result.Audio)
}

// writeTurnMultipart は Turn / StartTurn の応答を multipart/mixed で書く。
// 1つ目が JSON のメタ情報、2つ目が音声。
func writeTurnMultipart(ctx echo.Context, meta map[string]any, audio []byte) error {
	mw := multipart.NewWriter(ctx.Response().Writer)
	ctx.Response().Header().Set("Content-Type", "multipart/mixed; boundary="+mw.Boundary())

	metaPart, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json"}})
	json.NewEncoder(metaPart).Encode(meta)

	audioPart, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {audioContentType(audio)}})
	audioPart.Write(audio)
	mw.Close()
	return nil
}

func parseCompanyQuestionHints(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	if len(raw) > 16*1024 {
		return nil, fmt.Errorf("company question hints are too large")
	}
	var hints []string
	if err := json.Unmarshal([]byte(raw), &hints); err != nil {
		return nil, fmt.Errorf("invalid company question hints: %w", err)
	}
	if err := validateCompanyQuestionHints(hints); err != nil {
		return nil, err
	}
	return hints, nil
}

func validateCompanyQuestionHints(hints []string) error {
	if len(hints) > 5 {
		return fmt.Errorf("too many company question hints")
	}
	for _, hint := range hints {
		if utf8.RuneCountInString(hint) > 300 {
			return fmt.Errorf("company question hint is too long")
		}
	}
	return nil
}

// Create POST /api/interviews
func (c *InterviewController) Create(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	var req interviewCreateRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	resp, err := c.interviewService.CreateSession(userID, req.Language, req.InterviewerGender)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusOK, resp)
}

// Start POST /api/interviews/:id/start
func (c *InterviewController) Start(ctx echo.Context) error {
	sessionID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid interview id")
	}
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	resp, err := c.interviewService.StartSession(userID, sessionID)
	if err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusOK, resp)
}

// Finish POST /api/interviews/:id/finish
func (c *InterviewController) Finish(ctx echo.Context) error {
	sessionID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid interview id")
	}
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	resp, err := c.interviewService.FinishSession(userID, sessionID)
	if err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusOK, resp)
}

// List GET /api/interviews
func (c *InterviewController) List(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	page := httpapi.IntQuery(ctx, "page", 1)
	limit := httpapi.LimitQuery(ctx, "limit", 20)
	offset := (page - 1) * limit
	allStr := ctx.QueryParam("all")
	all := allStr == "1" || strings.ToLower(allStr) == "true"
	sessions, total, err := c.interviewService.ListSessions(userID, all, limit, offset)
	if err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"sessions": sessions,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

// HRList GET /api/hr/interviews?company_id= - 企業オーナー向け面接一覧（#1083）
func (c *InterviewController) HRList(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	companyID, err := httpapi.RequiredUintQuery(ctx, "company_id")
	if err != nil {
		return err
	}
	page := httpapi.IntQuery(ctx, "page", 1)
	limit := httpapi.LimitQuery(ctx, "limit", 20)
	offset := (page - 1) * limit
	sessions, total, err := c.interviewService.ListSessionsForOwner(userID, companyID, limit, offset)
	if err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"sessions": sessions,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

// Get GET /api/interviews/:id
func (c *InterviewController) Get(ctx echo.Context) error {
	sessionID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid interview id")
	}
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	// 教員用レポートの開示可否は実ロールからサービス側で決める。role クエリは認可に使わない(#1814)。
	resp, err := c.interviewService.GetSessionDetail(userID, sessionID)
	if err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusOK, resp)
}

// AddUtterance POST /api/interviews/:id/utterances
func (c *InterviewController) AddUtterance(ctx echo.Context) error {
	sessionID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid interview id")
	}
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	var req interviewUtteranceRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if err := c.interviewService.SaveUtterance(userID, sessionID, req.Role, req.Text, req.ClientUtteranceID); err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.NoContent(http.StatusNoContent)
}

// parseFormInt はフォーム値を整数として取得し、失敗時はデフォルト値を返す。
func parseFormInt(r interface{ FormValue(string) string }, key string, def int) int {
	v := r.FormValue(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}

// audioContentType は音声の中身から Content-Type を決める（#1833）。
// OpenAI の TTS は mp3、OpenAI 障害時のローカル（VOICEVOX）は WAV を返す。
// ブラウザはどちらもそのまま再生できるが、Content-Type が中身と違うと再生に失敗する環境がある。
func audioContentType(audio []byte) string {
	if aifailover.IsWAV(audio) {
		return "audio/wav"
	}
	return "audio/mpeg"
}
