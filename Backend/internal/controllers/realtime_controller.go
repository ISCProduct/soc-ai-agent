package controllers

import (
	"Backend/internal/services"
	"encoding/json"
	"net/http"
	"strings"
)

type RealtimeController struct {
	interviewService     *services.InterviewService
	realtimeUsageService *services.RealtimeUsageService
}

func NewRealtimeController(interviewService *services.InterviewService, realtimeUsageService *services.RealtimeUsageService) *RealtimeController {
	return &RealtimeController{interviewService: interviewService, realtimeUsageService: realtimeUsageService}
}

type realtimeTokenRequest struct {
	UserID      uint `json:"user_id"`
	InterviewID uint `json:"interview_id"`
}

type realtimeTokenResponse struct {
	ClientSecret string `json:"client_secret"`
}

func (c *RealtimeController) Token(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req realtimeTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserID == 0 || req.InterviewID == 0 {
		http.Error(w, "user_id and interview_id are required", http.StatusBadRequest)
		return
	}
	secret, err := c.interviewService.CreateRealtimeToken(r.Context(), req.UserID, req.InterviewID)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		if strings.Contains(err.Error(), "realtime capacity exceeded") {
			status = http.StatusTooManyRequests
		}
		writeErrorByStatus(w, status, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(realtimeTokenResponse{ClientSecret: secret})
}

type sessionInfoResponse struct {
	SessionMinutes int `json:"session_minutes"`
}

// SessionInfo はユーザー向けのセッション時間（分）を返す。コスト情報は含まない。
func (c *RealtimeController) SessionInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	minutes := 10
	if c.realtimeUsageService != nil {
		minutes = c.realtimeUsageService.SessionDurationMinutes()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessionInfoResponse{SessionMinutes: minutes})
}
