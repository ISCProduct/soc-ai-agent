package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/services"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// AdminInterviewController provides admin endpoints for viewing interview sessions and videos.
type AdminInterviewController struct {
	interviewService    *services.InterviewService
	videoRepo           repository.InterviewVideoRepository
	s3Service           *services.S3UploadService
	companyQuestionRepo repository.InterviewCompanyQuestionRepository
}

func NewAdminInterviewController(
	interviewService *services.InterviewService,
	videoRepo repository.InterviewVideoRepository,
	s3Service *services.S3UploadService,
) *AdminInterviewController {
	return &AdminInterviewController{
		interviewService: interviewService,
		videoRepo:        videoRepo,
		s3Service:        s3Service,
	}
}

// SetCompanyQuestionRepo 企業別質問リポジトリを注入する
func (c *AdminInterviewController) SetCompanyQuestionRepo(r repository.InterviewCompanyQuestionRepository) {
	c.companyQuestionRepo = r
}

// ListCompanyQuestions GET /api/admin/companies/:id/interview-questions
func (c *AdminInterviewController) ListCompanyQuestions(ctx echo.Context) error {
	if c.companyQuestionRepo == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not configured")
	}
	companyID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}
	qs, err := c.companyQuestionRepo.FindByCompanyID(companyID)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"questions": qs})
}

// CreateCompanyQuestion POST /api/admin/companies/:id/interview-questions
func (c *AdminInterviewController) CreateCompanyQuestion(ctx echo.Context) error {
	if c.companyQuestionRepo == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not configured")
	}
	companyID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}
	var req struct {
		Position     string `json:"position"`
		Category     string `json:"category"`
		QuestionText string `json:"question_text"`
		Priority     int    `json:"priority"`
		IsRequired   bool   `json:"is_required"`
	}
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	req.Position = strings.TrimSpace(req.Position)
	req.Category = strings.TrimSpace(req.Category)
	req.QuestionText = strings.TrimSpace(req.QuestionText)
	if req.Category == "" || req.QuestionText == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "category and question_text are required")
	}
	q := &models.InterviewCompanyQuestion{
		CompanyID:    companyID,
		Position:     req.Position,
		Category:     req.Category,
		QuestionText: req.QuestionText,
		Priority:     req.Priority,
		IsRequired:   req.IsRequired,
	}
	if err := c.companyQuestionRepo.Create(q); err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusCreated, q)
}

// UpdateCompanyQuestion PUT /api/admin/companies/:id/interview-questions/:qid
func (c *AdminInterviewController) UpdateCompanyQuestion(ctx echo.Context) error {
	if c.companyQuestionRepo == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not configured")
	}
	companyID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}
	qID, err := echoUintParam(ctx, "qid")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid question ID")
	}
	q, err := c.companyQuestionRepo.FindByID(qID)
	if err != nil || q == nil || q.CompanyID != companyID {
		return echo.NewHTTPError(http.StatusNotFound, "Question not found")
	}
	var req struct {
		Position     *string `json:"position"`
		Category     *string `json:"category"`
		QuestionText *string `json:"question_text"`
		Priority     *int    `json:"priority"`
		IsRequired   *bool   `json:"is_required"`
	}
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if req.Position != nil {
		q.Position = strings.TrimSpace(*req.Position)
	}
	if req.Category != nil {
		category := strings.TrimSpace(*req.Category)
		if category == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "category must not be empty")
		}
		q.Category = category
	}
	if req.QuestionText != nil {
		questionText := strings.TrimSpace(*req.QuestionText)
		if questionText == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "question_text must not be empty")
		}
		q.QuestionText = questionText
	}
	if req.Priority != nil {
		q.Priority = *req.Priority
	}
	if req.IsRequired != nil {
		q.IsRequired = *req.IsRequired
	}
	if err := c.companyQuestionRepo.Update(q); err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, q)
}

// DeleteCompanyQuestion DELETE /api/admin/companies/:id/interview-questions/:qid
func (c *AdminInterviewController) DeleteCompanyQuestion(ctx echo.Context) error {
	if c.companyQuestionRepo == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not configured")
	}
	companyID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}
	qID, err := echoUintParam(ctx, "qid")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid question ID")
	}
	q, err := c.companyQuestionRepo.FindByID(qID)
	if err != nil || q == nil || q.CompanyID != companyID {
		return echo.NewHTTPError(http.StatusNotFound, "Question not found")
	}
	if err := c.companyQuestionRepo.Delete(qID); err != nil {
		return echoInternalError(err)
	}
	return ctx.NoContent(http.StatusNoContent)
}

// ListSessions handles GET /api/admin/interviews
// Returns all interview sessions with pagination.
func (c *AdminInterviewController) ListSessions(ctx echo.Context) error {
	page := echoIntQuery(ctx, "page", 1)
	limit := echoIntQuery(ctx, "limit", 20)
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	sessions, total, err := c.interviewService.ListAllSessionsAdmin(limit, offset)
	if err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"sessions": sessions,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

// ListVideos handles GET /api/admin/interviews/:id/videos
func (c *AdminInterviewController) ListVideos(ctx echo.Context) error {
	sessionID, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}

	videos, err := c.videoRepo.FindBySessionID(ctx.Request().Context(), uint(sessionID))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch videos")
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"videos": videos,
	})
}

// VideoURL handles GET /api/admin/interviews/:id/videos/:video_id/url
// Returns a presigned S3 URL valid for 15 minutes.
func (c *AdminInterviewController) VideoURL(ctx echo.Context) error {
	videoID, err := strconv.ParseUint(ctx.Param("video_id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid video ID")
	}

	video, err := c.videoRepo.FindByID(ctx.Request().Context(), uint(videoID))
	if err != nil || video == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Video not found")
	}

	if video.Status != "done" || video.DriveFileID == "" {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "Video is not available yet")
	}

	if c.s3Service == nil {
		// S3 not configured: return the stored URL directly
		return ctx.JSON(http.StatusOK, map[string]string{
			"url":        video.DriveFileURL,
			"expires_at": "",
		})
	}

	expires := 15 * time.Minute
	presignedURL, err := c.s3Service.PresignGetURL(ctx.Request().Context(), video.DriveFileID, expires)
	if err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]string{
		"url":        presignedURL,
		"expires_at": time.Now().Add(expires).UTC().Format(time.RFC3339),
	})
}
