package controllers

import (
	"Backend/internal/middleware"
	"Backend/internal/services"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"
)

type ResumeController struct {
	resumeService *services.ResumeService
}

func NewResumeController(resumeService *services.ResumeService) *ResumeController {
	return &ResumeController{resumeService: resumeService}
}

func authenticatedUserID(r *http.Request) (uint, error) {
	userID, ok := r.Context().Value(middleware.UserIDContextKey).(uint)
	if !ok || userID == 0 {
		return 0, errors.New("user_id is required")
	}
	return userID, nil
}

func (c *ResumeController) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	userIDStr := r.FormValue("user_id")
	sessionID := r.FormValue("session_id")
	sourceType := r.FormValue("source_type")
	sourceURL := r.FormValue("source_url")

	if userIDStr == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid user_id", http.StatusBadRequest)
		return
	}
	authenticatedID, err := authenticatedUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if authenticatedID != uint(userID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var fileHeader *multipart.FileHeader
	file, header, err := r.FormFile("file")
	if err == nil {
		file.Close()
		fileHeader = header
	}

	result, err := c.resumeService.Upload(uint(userID), sessionID, sourceType, sourceURL, fileHeader)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (c *ResumeController) Review(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	docIDStr := r.URL.Query().Get("document_id")
	if docIDStr == "" {
		http.Error(w, "document_id is required", http.StatusBadRequest)
		return
	}
	docID, err := strconv.ParseUint(docIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid document_id", http.StatusBadRequest)
		return
	}
	userID, err := authenticatedUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if err := c.resumeService.EnsureDocumentOwner(uint(docID), userID); err != nil {
		if errors.Is(err, services.ErrForbidden) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		writeInternalServerError(w, err)
		return
	}

	var payload struct {
		CompanyName   string `json:"company_name"`
		CandidateType string `json:"candidate_type"`
		JobTitle      string `json:"job_title"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
	}

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
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			http.Error(w, ve.Message, http.StatusUnprocessableEntity)
			return
		}
		writeInternalServerError(w, err)
		return
	}
	log.Printf("resume_review: completed document_id=%d score=%d items=%d", docID, review.Score, len(items))

	resp := map[string]interface{}{
		"review": review,
		"items":  items,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (c *ResumeController) ReviewStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	docIDStr := r.URL.Query().Get("document_id")
	if docIDStr == "" {
		http.Error(w, "document_id is required", http.StatusBadRequest)
		return
	}
	docID, err := strconv.ParseUint(docIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid document_id", http.StatusBadRequest)
		return
	}
	userID, err := authenticatedUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if err := c.resumeService.EnsureDocumentOwner(uint(docID), userID); err != nil {
		if errors.Is(err, services.ErrForbidden) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		writeInternalServerError(w, err)
		return
	}

	var payload struct {
		CompanyName   string `json:"company_name"`
		CandidateType string `json:"candidate_type"`
		JobTitle      string `json:"job_title"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
	}

	log.Printf(
		"resume_review_stream: start document_id=%d company=%q job_title=%q",
		docID, payload.CompanyName, payload.JobTitle,
	)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	if err := c.resumeService.ReviewDocumentStream(r.Context(), uint(docID), userID, payload.CompanyName, payload.JobTitle, payload.CandidateType, w); err != nil {
		log.Printf("resume_review_stream: error document_id=%d err=%v", docID, err)
	}
}

func (c *ResumeController) Annotated(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	docIDStr := r.URL.Query().Get("document_id")
	if docIDStr == "" {
		http.Error(w, "document_id is required", http.StatusBadRequest)
		return
	}
	docID, err := strconv.ParseUint(docIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid document_id", http.StatusBadRequest)
		return
	}
	userID, err := authenticatedUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	file, err := c.resumeService.OpenAnnotatedFile(uint(docID), userID)
	if err != nil {
		if errors.Is(err, services.ErrForbidden) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if file.CloseFunc != nil {
		defer file.CloseFunc()
	}

	w.Header().Set("Content-Type", file.ContentType)
	if file.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))
	}
	http.ServeContent(w, r, file.Filename, time.Now(), file.Reader)
}
