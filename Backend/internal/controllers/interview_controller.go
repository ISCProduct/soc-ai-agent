package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/services"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"
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
	UserID            uint   `json:"user_id"`
	Language          string `json:"language"`
	InterviewerGender string `json:"interviewer_gender"`
}

type interviewActionRequest struct {
	UserID uint `json:"user_id"`
}

type interviewUtteranceRequest struct {
	UserID uint   `json:"user_id"`
	Role   string `json:"role"`
	Text   string `json:"text"`
}

func (c *InterviewController) ListOrCreate(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c.List(w, r)
	case http.MethodPost:
		c.Create(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (c *InterviewController) Route(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/interviews/")
	if strings.HasSuffix(path, "/start") {
		c.Start(w, r)
		return
	}
	if strings.HasSuffix(path, "/finish") {
		c.Finish(w, r)
		return
	}
	if strings.HasSuffix(path, "/utterances") {
		c.AddUtterance(w, r)
		return
	}
	if strings.HasSuffix(path, "/turn") {
		c.Turn(w, r)
		return
	}
	if strings.HasSuffix(path, "/start-turn") {
		c.StartTurn(w, r)
		return
	}
	if strings.HasSuffix(path, "/report") {
		c.GetReport(w, r)
		return
	}
	if strings.HasSuffix(path, "/send-report") {
		c.SendReport(w, r)
		return
	}
	if strings.HasSuffix(path, "/upload-video") {
		c.UploadVideo(w, r)
		return
	}
	if strings.HasSuffix(path, "/phrase-suggestions") {
		c.GetPhraseSuggestions(w, r)
		return
	}
	c.Get(w, r)
}

// GetTrend は GET /api/interviews/trend?user_id=X&limit=N を処理し、
// 完了済みセッションのスコア時系列を返す。
func (c *InterviewController) GetTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		http.Error(w, "invalid user_id", http.StatusBadRequest)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	points, err := c.interviewService.GetTrend(uint(userID), limit)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"points": points,
	})
}

func (c *InterviewController) GetReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID, err := extractID(r.URL.Path, "/api/interviews/", "/report")
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		http.Error(w, "invalid user_id", http.StatusBadRequest)
		return
	}
	report, err := c.interviewService.GetReport(uint(userID), sessionID)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		writeErrorByStatus(w, status, err)
		return
	}
	if report == nil {
		http.Error(w, "report not yet available", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

func (c *InterviewController) GetPhraseSuggestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID, err := extractID(r.URL.Path, "/api/interviews/", "/phrase-suggestions")
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		http.Error(w, "invalid user_id", http.StatusBadRequest)
		return
	}
	suggestions, err := c.interviewService.GetPhraseSuggestions(r.Context(), uint(userID), sessionID)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		writeErrorByStatus(w, status, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"suggestions": suggestions,
	})
}

func (c *InterviewController) SendReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID, err := extractID(r.URL.Path, "/api/interviews/", "/send-report")
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}
	var req struct {
		UserID uint `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == 0 {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	if err := c.interviewService.SendReportEmail(req.UserID, sessionID); err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "user not found" || err.Error() == "report not found" {
			status = http.StatusNotFound
		}
		if err.Error() == "guest users cannot receive email reports" {
			status = http.StatusForbidden
		}
		writeErrorByStatus(w, status, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "レポートをメールで送信しました"})
}

// maxVideoSize は受け付ける動画の最大サイズ（500 MB）
const maxVideoSize = 500 << 20

func (c *InterviewController) UploadVideo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID, err := extractID(r.URL.Path, "/api/interviews/", "/upload-video")
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	if c.videoRepo == nil || c.s3Service == nil {
		http.Error(w, "video upload service not configured", http.StatusServiceUnavailable)
		return
	}

	// メモリには最大 10 MB を確保し、それ以上は一時ファイルに書き出す
	// これにより 32 MB 超の動画もメモリ不足なく受信できる
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "リクエストの解析に失敗しました。ファイルが破損しているか、サイズが大きすぎます", http.StatusBadRequest)
		return
	}

	userIDStr := r.FormValue("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil || userID == 0 {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		http.Error(w, "動画ファイルが見つかりません", http.StatusBadRequest)
		return
	}

	// ファイルサイズ上限チェック
	if header.Size > maxVideoSize {
		file.Close()
		http.Error(w, fmt.Sprintf("動画ファイルが大きすぎます（上限 %d MB）。録画設定を下げて再度お試しください", maxVideoSize>>20), http.StatusRequestEntityTooLarge)
		return
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
		UserID:        uint(userID),
		FileName:      fileName,
		FileSizeBytes: header.Size,
		MimeType:      mimeType,
		Status:        "uploading",
	}
	if err := c.videoRepo.Create(r.Context(), videoRecord); err != nil {
		file.Close()
		http.Error(w, "動画レコードの作成に失敗しました", http.StatusInternalServerError)
		return
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"video_id": videoRecord.ID,
		"status":   "uploading",
		"message":  "動画のアップロードを開始しました",
	})
}

func (c *InterviewController) Turn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID, err := extractID(r.URL.Path, "/api/interviews/", "/turn")
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	// multipart から音声と履歴を取得
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}
	userIDStr := r.FormValue("user_id")
	var userID uint
	if id, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
		userID = uint(id)
	}
	historyStr := r.FormValue("history")
	var history []map[string]string
	if historyStr != "" {
		json.Unmarshal([]byte(historyStr), &history)
	}
	companyName := r.FormValue("company_name")
	companyReading := r.FormValue("company_reading")
	position := r.FormValue("position")
	companyInfo := r.FormValue("company_info")
	companyType := r.FormValue("company_type")
	turnCount := 0
	if tc := r.FormValue("turn_count"); tc != "" {
		if parsed, err := strconv.Atoi(tc); err == nil && parsed >= 0 {
			turnCount = parsed
		}
	}
	remainingSeconds := 0
	if rs := r.FormValue("remaining_seconds"); rs != "" {
		if parsed, err := strconv.Atoi(rs); err == nil && parsed >= 0 {
			remainingSeconds = parsed
		}
	}
	questionIndex := 0
	if qi := r.FormValue("question_index"); qi != "" {
		if parsed, err := strconv.Atoi(qi); err == nil && parsed >= 0 {
			questionIndex = parsed
		}
	}
	totalQuestions := 0
	if tq := r.FormValue("total_questions"); tq != "" {
		if parsed, err := strconv.Atoi(tq); err == nil && parsed >= 0 {
			totalQuestions = parsed
		}
	}
	questionElapsedSeconds := 0
	if qe := r.FormValue("question_elapsed_seconds"); qe != "" {
		if parsed, err := strconv.Atoi(qe); err == nil && parsed >= 0 {
			questionElapsedSeconds = parsed
		}
	}
	questionDurationSeconds := 0
	if qd := r.FormValue("question_duration_seconds"); qd != "" {
		if parsed, err := strconv.Atoi(qd); err == nil && parsed >= 0 {
			questionDurationSeconds = parsed
		}
	}

	audioFile, _, err := r.FormFile("audio")
	if err != nil {
		http.Error(w, "audio file required", http.StatusBadRequest)
		return
	}
	defer audioFile.Close()
	audioData, err := io.ReadAll(audioFile)
	if err != nil {
		http.Error(w, "Failed to read audio", http.StatusInternalServerError)
		return
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
		writeInternalServerError(w, err)
		return
	}

	// multipart レスポンス: JSON メタ + audio
	mw := multipart.NewWriter(w)
	w.Header().Set("Content-Type", "multipart/mixed; boundary="+mw.Boundary())

	metaPart, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json"}})
	json.NewEncoder(metaPart).Encode(map[string]string{
		"user_text": result.UserText,
		"ai_text":   result.AIText,
	})

	audioPart, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"audio/mpeg"}})
	audioPart.Write(result.Audio)
	mw.Close()
}

func (c *InterviewController) StartTurn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID, err := extractID(r.URL.Path, "/api/interviews/", "/start-turn")
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	var req struct {
		UserID                  uint   `json:"user_id"`
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
	json.NewDecoder(r.Body).Decode(&req)

	result, err := c.interviewService.StartTurn(
		r.Context(),
		req.UserID,
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
		writeInternalServerError(w, err)
		return
	}

	mw := multipart.NewWriter(w)
	w.Header().Set("Content-Type", "multipart/mixed; boundary="+mw.Boundary())

	metaPart, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json"}})
	json.NewEncoder(metaPart).Encode(map[string]string{"ai_text": result.AIText})

	audioPart, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"audio/mpeg"}})
	audioPart.Write(result.Audio)
	mw.Close()
}

func (c *InterviewController) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req interviewCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserID == 0 {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	resp, err := c.interviewService.CreateSession(req.UserID, req.Language, req.InterviewerGender)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (c *InterviewController) Start(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID, err := extractID(r.URL.Path, "/api/interviews/", "/start")
	if err != nil {
		http.Error(w, "invalid interview id", http.StatusBadRequest)
		return
	}
	var req interviewActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserID == 0 {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	resp, err := c.interviewService.StartSession(req.UserID, sessionID)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		writeErrorByStatus(w, status, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (c *InterviewController) Finish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID, err := extractID(r.URL.Path, "/api/interviews/", "/finish")
	if err != nil {
		http.Error(w, "invalid interview id", http.StatusBadRequest)
		return
	}
	var req interviewActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserID == 0 {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	resp, err := c.interviewService.FinishSession(req.UserID, sessionID)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		writeErrorByStatus(w, status, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (c *InterviewController) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		http.Error(w, "invalid user_id", http.StatusBadRequest)
		return
	}
	page := parseIntQuery(r, "page", 1)
	limit := parseIntQuery(r, "limit", 20)
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit
	all := r.URL.Query().Get("all") == "1" || strings.ToLower(r.URL.Query().Get("all")) == "true"
	sessions, total, err := c.interviewService.ListSessions(uint(userID), all, limit, offset)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		writeErrorByStatus(w, status, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessions": sessions,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

func (c *InterviewController) Get(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID, err := extractID(r.URL.Path, "/api/interviews/", "")
	if err != nil {
		http.Error(w, "invalid interview id", http.StatusBadRequest)
		return
	}
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		http.Error(w, "invalid user_id", http.StatusBadRequest)
		return
	}
	role := r.URL.Query().Get("role")
	if role == "" {
		role = "student"
	}
	resp, err := c.interviewService.GetSessionDetailWithRole(uint(userID), sessionID, role)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		writeErrorByStatus(w, status, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (c *InterviewController) AddUtterance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID, err := extractID(r.URL.Path, "/api/interviews/", "/utterances")
	if err != nil {
		http.Error(w, "invalid interview id", http.StatusBadRequest)
		return
	}
	var req interviewUtteranceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserID == 0 {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	if err := c.interviewService.SaveUtterance(req.UserID, sessionID, req.Role, req.Text); err != nil {
		status := http.StatusBadRequest
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		writeErrorByStatus(w, status, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func extractID(path string, prefix string, suffix string) (uint, error) {
	trimmed := strings.TrimPrefix(path, prefix)
	if suffix != "" && strings.HasSuffix(trimmed, suffix) {
		trimmed = strings.TrimSuffix(trimmed, suffix)
	}
	trimmed = strings.Trim(trimmed, "/")
	id, err := strconv.ParseUint(trimmed, 10, 32)
	if err != nil {
		return 0, err
	}
	if id > math.MaxInt32 {
		return 0, fmt.Errorf("id %d exceeds maximum allowed value", id)
	}
	return uint(id), nil
}

func parseIntQuery(r *http.Request, key string, def int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return def
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
