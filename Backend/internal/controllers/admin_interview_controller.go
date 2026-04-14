package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/services"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
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
func (c *AdminInterviewController) ListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	page := parseIntQuery(r, "page", 1)
	limit := parseIntQuery(r, "limit", 20)
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	sessions, total, err := c.interviewService.ListAllSessionsAdmin(limit, offset)
	if err != nil {
		writeInternalServerError(w, err)
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

// ListVideos handles GET /api/admin/interviews/{id}/videos
func (c *AdminInterviewController) ListVideos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID, err := extractAdminInterviewID(r.URL.Path, "/videos")
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	videos, err := c.videoRepo.FindBySessionID(r.Context(), sessionID)
	if err != nil {
		http.Error(w, "Failed to fetch videos", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"videos": videos,
	})
}

// VideoURL handles GET /api/admin/interviews/{id}/videos/{video_id}/url
// Returns a presigned S3 URL valid for 15 minutes.
func (c *AdminInterviewController) VideoURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	videoID, err := extractVideoID(r.URL.Path)
	if err != nil {
		http.Error(w, "Invalid video ID", http.StatusBadRequest)
		return
	}

	video, err := c.videoRepo.FindByID(r.Context(), videoID)
	if err != nil || video == nil {
		http.Error(w, "Video not found", http.StatusNotFound)
		return
	}

	if video.Status != "done" || video.DriveFileID == "" {
		http.Error(w, "Video is not available yet", http.StatusUnprocessableEntity)
		return
	}

	if c.s3Service == nil {
		// S3 not configured: return the stored URL directly
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"url":        video.DriveFileURL,
			"expires_at": "",
		})
		return
	}

	expires := 15 * time.Minute
	presignedURL, err := c.s3Service.PresignGetURL(r.Context(), video.DriveFileID, expires)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url":        presignedURL,
		"expires_at": time.Now().Add(expires).UTC().Format(time.RFC3339),
	})
}

// Route dispatches /api/admin/interviews/ sub-paths.
func (c *AdminInterviewController) Route(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// /api/admin/interviews/{id}/videos/{video_id}/url
	if strings.HasSuffix(path, "/url") && strings.Contains(path, "/videos/") {
		c.VideoURL(w, r)
		return
	}
	// /api/admin/interviews/{id}/videos
	if strings.HasSuffix(path, "/videos") {
		c.ListVideos(w, r)
		return
	}
	http.Error(w, "not found", http.StatusNotFound)
}

func extractAdminInterviewID(path, suffix string) (uint, error) {
	// /api/admin/interviews/{id}/videos → extract {id}
	trimmed := strings.TrimPrefix(path, "/api/admin/interviews/")
	if idx := strings.Index(trimmed, "/"); idx != -1 {
		trimmed = trimmed[:idx]
	}
	trimmed = strings.TrimSuffix(trimmed, suffix)
	id, err := strconv.ParseUint(trimmed, 10, 32)
	return uint(id), err
}

func extractVideoID(path string) (uint, error) {
	// /api/admin/interviews/{id}/videos/{video_id}/url
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// parts: ["api","admin","interviews","{id}","videos","{video_id}","url"]
	for i, p := range parts {
		if p == "videos" && i+1 < len(parts) {
			vidStr := parts[i+1]
			// remove trailing /url if present
			vidStr = strings.TrimSuffix(vidStr, "/url")
			id, err := strconv.ParseUint(vidStr, 10, 32)
			return uint(id), err
		}
	}
	return 0, strconv.ErrSyntax
}
