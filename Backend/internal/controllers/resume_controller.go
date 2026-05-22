package controllers

import (
	"Backend/internal/services"
	"errors"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

type ResumeController struct {
	resumeService *services.ResumeService
}

func NewResumeController(resumeService *services.ResumeService) *ResumeController {
	return &ResumeController{resumeService: resumeService}
}

func (c *ResumeController) Upload(ctx echo.Context) error {
	if err := ctx.Request().ParseMultipartForm(32 << 20); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid form data")
	}

	userIDStr := ctx.FormValue("user_id")
	sessionID := ctx.FormValue("session_id")
	sourceType := ctx.FormValue("source_type")
	sourceURL := ctx.FormValue("source_url")

	if userIDStr == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user_id is required")
	}
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid user_id")
	}
	authenticatedID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "user_id is required")
	}
	if authenticatedID != uint(userID) {
		return echo.NewHTTPError(http.StatusForbidden, "forbidden")
	}

	var fileHeader *multipart.FileHeader
	file, header, err := ctx.Request().FormFile("file")
	if err == nil {
		file.Close()
		fileHeader = header
	}

	result, err := c.resumeService.Upload(uint(userID), sessionID, sourceType, sourceURL, fileHeader)
	if err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, result)
}

func (c *ResumeController) Review(ctx echo.Context) error {
	docIDStr := ctx.QueryParam("document_id")
	if docIDStr == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "document_id is required")
	}
	docID, err := strconv.ParseUint(docIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid document_id")
	}
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "user_id is required")
	}
	if err := c.resumeService.EnsureDocumentOwner(uint(docID), userID); err != nil {
		if errors.Is(err, services.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, "forbidden")
		}
		return echoInternalError(err)
	}

	var payload struct {
		CompanyName   string `json:"company_name"`
		CandidateType string `json:"candidate_type"`
		JobTitle      string `json:"job_title"`
	}
	ctx.Bind(&payload)

	log.Printf(
		"resume_review: start document_id=%d company=%q job_title=%q candidate_type=%q",
		docID,
		payload.CompanyName,
		payload.JobTitle,
		payload.CandidateType,
	)

	review, items, err := c.resumeService.ReviewDocument(uint(docID), userID, payload.CompanyName, payload.JobTitle, payload.CandidateType)
	if err != nil {
		log.Printf("resume_review: failed document_id=%d err=%v", docID, err)
		if errors.Is(err, services.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, "forbidden")
		}
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, ve.Message)
		}
		return echoInternalError(err)
	}
	log.Printf("resume_review: completed document_id=%d score=%d items=%d", docID, review.Score, len(items))

	return ctx.JSON(http.StatusOK, map[string]any{
		"review": review,
		"items":  items,
	})
}

func (c *ResumeController) ReviewStream(ctx echo.Context) error {
	docIDStr := ctx.QueryParam("document_id")
	if docIDStr == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "document_id is required")
	}
	docID, err := strconv.ParseUint(docIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid document_id")
	}
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "user_id is required")
	}
	if err := c.resumeService.EnsureDocumentOwner(uint(docID), userID); err != nil {
		if errors.Is(err, services.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, "forbidden")
		}
		return echoInternalError(err)
	}

	var payload struct {
		CompanyName   string `json:"company_name"`
		CandidateType string `json:"candidate_type"`
		JobTitle      string `json:"job_title"`
	}
	ctx.Bind(&payload)

	log.Printf(
		"resume_review_stream: start document_id=%d company=%q job_title=%q",
		docID, payload.CompanyName, payload.JobTitle,
	)

	w := ctx.Response().Writer
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	if err := c.resumeService.ReviewDocumentStream(ctx.Request().Context(), uint(docID), userID, payload.CompanyName, payload.JobTitle, payload.CandidateType, w); err != nil {
		log.Printf("resume_review_stream: error document_id=%d err=%v", docID, err)
	}
	return nil
}

func (c *ResumeController) Annotated(ctx echo.Context) error {
	docIDStr := ctx.QueryParam("document_id")
	if docIDStr == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "document_id is required")
	}
	docID, err := strconv.ParseUint(docIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid document_id")
	}
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "user_id is required")
	}

	file, err := c.resumeService.OpenAnnotatedFile(uint(docID), userID)
	if err != nil {
		if errors.Is(err, services.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, "forbidden")
		}
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	if file.CloseFunc != nil {
		defer file.CloseFunc()
	}

	w := ctx.Response().Writer
	r := ctx.Request()
	w.Header().Set("Content-Type", file.ContentType)
	if file.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))
	}
	http.ServeContent(w, r, file.Filename, time.Now(), file.Reader)
	return nil
}
