package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/services"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

type InterviewController struct {
	interviewService *services.InterviewService
	videoRepo        repository.InterviewVideoRepository
	s3Service        *services.S3UploadService
}

func NewInterviewController(interviewService *services.InterviewService, videoRepo repository.InterviewVideoRepository, s3Service *services.S3UploadService) *InterviewController {
	return &InterviewController{
		interviewService: interviewService,
		videoRepo:        videoRepo,
		s3Service:        s3Service,
	}
}

type interviewCreateRequest struct {
	Language          string `json:"language"`
	InterviewerGender string `json:"interviewer_gender"`
}

type interviewUtteranceRequest struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// GetTrend GET /api/interviews/trend?limit=N
// 完了済みセッションのスコア時系列を返す。
func (c *InterviewController) GetTrend(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	limit := echoIntQuery(ctx, "limit", 20)
	points, err := c.interviewService.GetTrend(userID, limit)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"points": points,
	})
}

// GetReport GET /api/interviews/:id/report
func (c *InterviewController) GetReport(ctx echo.Context) error {
	sessionID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	report, err := c.interviewService.GetReport(userID, sessionID)
	if err != nil {
		if err.Error() == "forbidden" {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		return echoInternalError(err)
	}
	if report == nil {
		return echo.NewHTTPError(http.StatusNotFound, "report not yet available")
	}
	return ctx.JSON(http.StatusOK, report)
}

// GetPhraseSuggestions GET /api/interviews/:id/phrase-suggestions
func (c *InterviewController) GetPhraseSuggestions(ctx echo.Context) error {
	sessionID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	suggestions, err := c.interviewService.GetPhraseSuggestions(ctx.Request().Context(), userID, sessionID)
	if err != nil {
		if err.Error() == "forbidden" {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"suggestions": suggestions,
	})
}

// SendReport POST /api/interviews/:id/send-report
func (c *InterviewController) SendReport(ctx echo.Context) error {
	sessionID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	if err := c.interviewService.SendReportEmail(userID, sessionID); err != nil {
		if err.Error() == "user not found" || err.Error() == "report not found" {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		if err.Error() == "guest users cannot receive email reports" {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]string{"message": "レポートをメールで送信しました"})
}

// maxVideoSize は受け付ける動画の最大サイズ（500 MB）
const maxVideoSize = 500 << 20

// UploadVideo POST /api/interviews/:id/upload-video
func (c *InterviewController) UploadVideo(ctx echo.Context) error {
	sessionID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}

	if c.videoRepo == nil || c.s3Service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "video upload service not configured")
	}

	// メモリには最大 10 MB を確保し、それ以上は一時ファイルに書き出す
	r := ctx.Request()
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "リクエストの解析に失敗しました。ファイルが破損しているか、サイズが大きすぎます")
	}

	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "動画ファイルが見つかりません")
	}

	// ファイルサイズ上限チェック
	if header.Size > maxVideoSize {
		file.Close()
		return echo.NewHTTPError(http.StatusRequestEntityTooLarge,
			fmt.Sprintf("動画ファイルが大きすぎます（上限 %d MB）。録画設定を下げて再度お試しください", maxVideoSize>>20))
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "video/webm"
	}

	now := time.Now()
	s3Key := fmt.Sprintf("interview-videos/%d/%d_%s.webm", sessionID, userID, now.Format("20060102_150405"))
	fileName := fmt.Sprintf("interview_%d_%d_%s.webm", sessionID, userID, now.Format("20060102_150405"))

	videoRecord := &models.InterviewVideo{
		SessionID:     sessionID,
		UserID:        userID,
		FileName:      fileName,
		FileSizeBytes: header.Size,
		MimeType:      mimeType,
		Status:        "uploading",
	}
	if err := c.videoRepo.Create(r.Context(), videoRecord); err != nil {
		file.Close()
		return echo.NewHTTPError(http.StatusInternalServerError, "動画レコードの作成に失敗しました")
	}

	// S3 へのアップロードを非同期で実行（io.Reader をそのまま渡してメモリを節約）
	go func(vid *models.InterviewVideo, f io.ReadCloser, key string) {
		defer f.Close()
		ctx := context.Background()
		fileID, s3URL, uploadErr := c.s3Service.UploadReader(ctx, key, vid.MimeType, f)
		uploadedAt := time.Now()
		if uploadErr != nil {
			c.videoRepo.UpdateStatus(ctx, vid.ID, "error", fmt.Sprintf("S3へのアップロードに失敗しました: %v", uploadErr), "", "", nil)
			return
		}
		c.videoRepo.UpdateStatus(ctx, vid.ID, "done", "", fileID, s3URL, &uploadedAt)
	}(videoRecord, file, s3Key)

	return ctx.JSON(http.StatusOK, map[string]any{
		"video_id": videoRecord.ID,
		"status":   "uploading",
		"message":  "動画のアップロードを開始しました",
	})
}

// Turn POST /api/interviews/:id/turn
func (c *InterviewController) Turn(ctx echo.Context) error {
	sessionID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}

	// multipart から音声と履歴を取得
	r := ctx.Request()
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Failed to parse form")
	}
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	historyStr := r.FormValue("history")
	var history []map[string]string
	if historyStr != "" {
		json.Unmarshal([]byte(historyStr), &history)
	}

	// フォームから各パラメータを取得
	companyName := r.FormValue("company_name")
	companyReading := r.FormValue("company_reading")
	position := r.FormValue("position")
	companyInfo := r.FormValue("company_info")
	companyType := r.FormValue("company_type")

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
		r.Context(),
		userID,
		sessionID,
		audioData,
		history,
		companyName,
		companyReading,
		position,
		companyInfo,
		companyType,
		turnCount,
		remainingSeconds,
		questionIndex,
		totalQuestions,
		questionElapsedSeconds,
		questionDurationSeconds,
	)
	if err != nil {
		return echoInternalError(err)
	}

	// multipart レスポンス: JSON メタ + audio
	w := ctx.Response().Writer
	mw := multipart.NewWriter(w)
	ctx.Response().Header().Set("Content-Type", "multipart/mixed; boundary="+mw.Boundary())

	metaPart, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json"}})
	json.NewEncoder(metaPart).Encode(map[string]string{
		"user_text": result.UserText,
		"ai_text":   result.AIText,
	})

	audioPart, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"audio/mpeg"}})
	audioPart.Write(result.Audio)
	mw.Close()
	return nil
}

// StartTurn POST /api/interviews/:id/start-turn
func (c *InterviewController) StartTurn(ctx echo.Context) error {
	sessionID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}

	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	var req struct {
		CompanyName             string `json:"company_name"`
		CompanyReading          string `json:"company_reading"`
		Position                string `json:"position"`
		CompanyInfo             string `json:"company_info"`
		CompanyType             string `json:"company_type"`
		QuestionIndex           int    `json:"question_index"`
		TotalQuestions          int    `json:"total_questions"`
		QuestionElapsedSeconds  int    `json:"question_elapsed_seconds"`
		QuestionDurationSeconds int    `json:"question_duration_seconds"`
	}
	ctx.Bind(&req)

	result, err := c.interviewService.StartTurn(
		ctx.Request().Context(),
		userID,
		sessionID,
		req.CompanyName,
		req.CompanyReading,
		req.Position,
		req.CompanyInfo,
		req.CompanyType,
		req.QuestionIndex,
		req.TotalQuestions,
		req.QuestionElapsedSeconds,
		req.QuestionDurationSeconds,
	)
	if err != nil {
		return echoInternalError(err)
	}

	w := ctx.Response().Writer
	mw := multipart.NewWriter(w)
	ctx.Response().Header().Set("Content-Type", "multipart/mixed; boundary="+mw.Boundary())

	metaPart, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json"}})
	json.NewEncoder(metaPart).Encode(map[string]string{"ai_text": result.AIText})

	audioPart, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"audio/mpeg"}})
	audioPart.Write(result.Audio)
	mw.Close()
	return nil
}

// Create POST /api/interviews
func (c *InterviewController) Create(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
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
	sessionID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid interview id")
	}
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	resp, err := c.interviewService.StartSession(userID, sessionID)
	if err != nil {
		if err.Error() == "forbidden" {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusOK, resp)
}

// Finish POST /api/interviews/:id/finish
func (c *InterviewController) Finish(ctx echo.Context) error {
	sessionID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid interview id")
	}
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	resp, err := c.interviewService.FinishSession(userID, sessionID)
	if err != nil {
		if err.Error() == "forbidden" {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusOK, resp)
}

// List GET /api/interviews
func (c *InterviewController) List(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	page := echoIntQuery(ctx, "page", 1)
	limit := echoIntQuery(ctx, "limit", 20)
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit
	allStr := ctx.QueryParam("all")
	all := allStr == "1" || strings.ToLower(allStr) == "true"
	sessions, total, err := c.interviewService.ListSessions(userID, all, limit, offset)
	if err != nil {
		if err.Error() == "forbidden" {
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

// Get GET /api/interviews/:id
func (c *InterviewController) Get(ctx echo.Context) error {
	sessionID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid interview id")
	}
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	role := ctx.QueryParam("role")
	if role == "" {
		role = "student"
	}
	resp, err := c.interviewService.GetSessionDetailWithRole(userID, sessionID, role)
	if err != nil {
		if err.Error() == "forbidden" {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusOK, resp)
}

// AddUtterance POST /api/interviews/:id/utterances
func (c *InterviewController) AddUtterance(ctx echo.Context) error {
	sessionID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid interview id")
	}
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	var req interviewUtteranceRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if err := c.interviewService.SaveUtterance(userID, sessionID, req.Role, req.Text); err != nil {
		if err.Error() == "forbidden" {
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
