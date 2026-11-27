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
	"time"
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

// CompareAxes GET /api/interviews/axis-comparison?base=&target=
// 2回の面接について、評価軸ごとの差分を返す。
func (c *InterviewController) CompareAxes(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	baseID, err := queryUint(ctx, "base")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "base にセッションIDを指定してください")
	}
	targetID, err := queryUint(ctx, "target")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "target にセッションIDを指定してください")
	}
	result, err := c.interviewService.CompareAxes(userID, baseID, targetID)
	if err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		if errors.Is(err, interviewsvc.ErrInvalidComparison) {
			return echo.NewHTTPError(http.StatusBadRequest, "異なる2つのセッションを指定してください")
		}
		if errors.Is(err, interviewsvc.ErrReportNotReady) {
			return echo.NewHTTPError(http.StatusNotFound, "report not yet available")
		}
		if errors.Is(err, repository.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "session not found")
		}
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, result)
}

func queryUint(ctx echo.Context, key string) (uint, error) {
	raw := ctx.QueryParam(key)
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, err
	}
	return uint(id), nil
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

// Playback GET /api/interviews/:id/videos
// 本人の録画一覧と、発話を録画時刻へ合わせるための目安を返す。保存先のキーは返さない。
func (c *InterviewController) Playback(ctx echo.Context) error {
	sessionID, userID, err := c.ownedSession(ctx)
	if err != nil {
		return err
	}
	detail, err := c.interviewService.GetSessionDetail(userID, sessionID)
	if err != nil {
		return interviewAccessError(err)
	}
	videos := []interviewsvc.StudentVideo{}
	if c.videoUpload != nil {
		videos, err = c.videoUpload.ListForUser(ctx.Request().Context(), sessionID, userID)
		if err != nil {
			return httpapi.InternalError(err)
		}
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"videos": videos,
		"cues":   interviewsvc.ReviewCues(detail.Session.StartedAt, detail.Utterances),
		"anchor": "session_started_at",
	})
}

// VideoPlaybackURL GET /api/interviews/:id/videos/:video_id/url
// 本人の完了済み録画にだけ、15分の再生URLを発行する。
func (c *InterviewController) VideoPlaybackURL(ctx echo.Context) error {
	sessionID, userID, err := c.ownedSession(ctx)
	if err != nil {
		return err
	}
	videoID, err := httpapi.UintParam(ctx, "video_id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid video ID")
	}
	if c.videoUpload == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "video playback is not configured")
	}
	if err := c.interviewService.EnsureSessionOwnership(userID, sessionID); err != nil {
		return interviewAccessError(err)
	}
	url, expiresAt, err := c.videoUpload.PresignForUser(ctx.Request().Context(), sessionID, videoID, userID)
	if err != nil {
		if errors.Is(err, interviewsvc.ErrVideoNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "video not found")
		}
		if errors.Is(err, interviewsvc.ErrVideoNotReady) {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "video is not available yet")
		}
		if errors.Is(err, interviewsvc.ErrVideoPlaybackUnavailable) {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "video playback is not configured")
		}
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]string{
		"url":        url,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}

func (c *InterviewController) ownedSession(ctx echo.Context) (uint, uint, error) {
	sessionID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return 0, 0, echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return 0, 0, echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	return sessionID, userID, nil
}

func interviewAccessError(err error) error {
	if errors.Is(err, shared.ErrForbidden) {
		return echo.NewHTTPError(http.StatusForbidden, "forbidden")
	}
	if errors.Is(err, repository.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "session not found")
	}
	return httpapi.InternalError(err)
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

	// テキスト回答（全音声失敗時の継続 #1831）。あれば音声は任意。
	answerText := strings.TrimSpace(r.FormValue("answer_text"))
	if utf8.RuneCountInString(answerText) > 2000 {
		return echo.NewHTTPError(http.StatusBadRequest, "answer_text too long")
	}

	var audioData []byte
	audioFile, _, err := r.FormFile("audio")
	if err != nil {
		if answerText == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "audio file required")
		}
	} else {
		defer audioFile.Close()
		audioData, err = io.ReadAll(audioFile)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to read audio")
		}
	}

	result, err := c.interviewService.Turn(
		interviewsvc.WithCompanyQuestionHints(r.Context(), questionHints),
		userID,
		sessionID,
		audioData,
		answerText,
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
		if errors.Is(err, interviewsvc.ErrSessionNotStarted) {
			return echo.NewHTTPError(http.StatusConflict, err.Error())
		}
		return httpapi.InternalError(err)
	}

	return writeTurnMultipart(ctx, map[string]any{
		"limit_reached":            result.LimitReached,
		"limit_reason":             result.LimitReason,
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
		if errors.Is(err, interviewsvc.ErrSessionNotStarted) {
			return echo.NewHTTPError(http.StatusConflict, err.Error())
		}
		return httpapi.InternalError(err)
	}

	return writeTurnMultipart(ctx, map[string]any{
		"limit_reached":            result.LimitReached,
		"limit_reason":             result.LimitReason,
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

	metaPart, err := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json"}})
	if err != nil {
		return err
	}
	if err := json.NewEncoder(metaPart).Encode(meta); err != nil {
		return err
	}

	audioPart, err := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {audioContentType(audio)}})
	if err != nil {
		return err
	}
	if _, err := audioPart.Write(audio); err != nil {
		return err
	}
	return mw.Close()
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
	// all=1（全件一覧）は廃止。指定されても本人分だけを返す（SOCAIAGENT-687）。
	sessions, total, err := c.interviewService.ListSessions(userID, limit, offset)
	if err != nil {
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
