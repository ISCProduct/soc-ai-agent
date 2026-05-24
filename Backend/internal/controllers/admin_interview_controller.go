package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/services"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

// AdminInterviewController provides admin endpoints for viewing interview sessions and videos.
type AdminInterviewController struct {
	interviewService *services.InterviewService
	videoRepo        repository.InterviewVideoRepository
	s3Service        *services.S3UploadService
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
