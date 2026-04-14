package controllers

import (
	"Backend/internal/repositories"
	"Backend/internal/services"
	"encoding/json"
	"net/http"
	"strconv"
)

// IntegratedProfileController ユーザー統合プロファイルAPI
type IntegratedProfileController struct {
	crossFeature         *services.CrossFeatureIntegrationService
	interviewSessionRepo *repositories.InterviewSessionRepository
	resumeRepo           *repositories.ResumeRepository
}

func NewIntegratedProfileController(
	crossFeature *services.CrossFeatureIntegrationService,
	interviewSessionRepo *repositories.InterviewSessionRepository,
	resumeRepo *repositories.ResumeRepository,
) *IntegratedProfileController {
	return &IntegratedProfileController{
		crossFeature:         crossFeature,
		interviewSessionRepo: interviewSessionRepo,
		resumeRepo:           resumeRepo,
	}
}

// GetProfile GET /api/user/profile?user_id=xxx&session_id=xxx
func (c *IntegratedProfileController) GetProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	userIDParsed, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		http.Error(w, "invalid user_id", http.StatusBadRequest)
		return
	}
	userID := uint(userIDParsed)

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}

	// 面接セッション数を取得
	interviewCount := 0
	if count, err := c.interviewSessionRepo.CountByUser(userID); err == nil {
		interviewCount = int(count)
	}

	// 職務経歴書レビュー完了有無を確認
	resumeReviewDone := false
	if docs, err := c.resumeRepo.FindDocumentsByUserID(userID); err == nil {
		for _, doc := range docs {
			if doc.Status == "reviewed" {
				resumeReviewDone = true
				break
			}
		}
	}

	profile, err := c.crossFeature.BuildIntegratedProfile(userID, sessionID, interviewCount, resumeReviewDone)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}
