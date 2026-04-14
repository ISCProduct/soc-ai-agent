package controllers

import (
	"Backend/internal/services"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// CollectiveInsightController 集合知レコメンドAPI
type CollectiveInsightController struct {
	svc *services.CollectiveInsightService
}

func NewCollectiveInsightController(svc *services.CollectiveInsightService) *CollectiveInsightController {
	return &CollectiveInsightController{svc: svc}
}

// Route /api/collective-insights/* のルーティング
func (c *CollectiveInsightController) Route(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/collective-insights")
	path = strings.Trim(path, "/")

	switch {
	case path == "recommendations" && r.Method == http.MethodGet:
		c.GetRecommendations(w, r)
	case path == "top-companies" && r.Method == http.MethodGet:
		c.GetTopPassRateCompanies(w, r)
	case path == "consent" && r.Method == http.MethodPut:
		c.UpdateConsent(w, r)
	case path == "actions" && r.Method == http.MethodPost:
		c.RecordAction(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// GetRecommendations GET /api/collective-insights/recommendations?user_id=xxx&session_id=xxx
// 類似スコアプロファイルのユーザーが通過した企業をレコメンドする
func (c *CollectiveInsightController) GetRecommendations(w http.ResponseWriter, r *http.Request) {
	userID, sessionID, ok := parseUserAndSession(r)
	if !ok {
		http.Error(w, "user_id and session_id are required", http.StatusBadRequest)
		return
	}

	// 除外企業IDをオプションで受け取る（カンマ区切り）
	var excludeIDs []uint
	if excStr := r.URL.Query().Get("exclude"); excStr != "" {
		for _, idStr := range strings.Split(excStr, ",") {
			if id, err := strconv.ParseUint(strings.TrimSpace(idStr), 10, 32); err == nil {
				excludeIDs = append(excludeIDs, uint(id))
			}
		}
	}

	items, err := c.svc.GetCollectiveRecommendations(userID, sessionID, excludeIDs)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}
	if items == nil {
		items = []services.CollectiveRecommendItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"recommendations": items,
		"count":           len(items),
	})
}

// GetTopPassRateCompanies GET /api/collective-insights/top-companies?limit=10
// 全ユーザー通過率の高い企業ランキング
func (c *CollectiveInsightController) GetTopPassRateCompanies(w http.ResponseWriter, r *http.Request) {
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	companies, err := c.svc.GetTopPassRateCompanies(limit)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"companies": companies,
	})
}

// UpdateConsent PUT /api/collective-insights/consent
// ユーザーの集合知参加同意を更新する
func (c *CollectiveInsightController) UpdateConsent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID uint `json:"user_id"`
		Allow  bool `json:"allow"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == 0 {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	if err := c.svc.UpdateConsent(req.UserID, req.Allow); err != nil {
		writeInternalServerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id": req.UserID,
		"allow":   req.Allow,
	})
}

// RecordAction POST /api/collective-insights/actions
// ユーザー行動を匿名ログとして記録する
func (c *CollectiveInsightController) RecordAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID     uint   `json:"user_id"`
		SessionID  string `json:"session_id"`
		CompanyID  uint   `json:"company_id"`
		ActionType string `json:"action_type"` // viewed / applied / passed / rejected
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserID == 0 || req.CompanyID == 0 || req.ActionType == "" {
		http.Error(w, "user_id, company_id, action_type are required", http.StatusBadRequest)
		return
	}

	validActions := map[string]bool{"viewed": true, "applied": true, "passed": true, "rejected": true}
	if !validActions[req.ActionType] {
		http.Error(w, "invalid action_type", http.StatusBadRequest)
		return
	}

	if err := c.svc.RecordAction(req.UserID, req.SessionID, req.CompanyID, req.ActionType); err != nil {
		writeInternalServerError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "recorded"})
}

// RebuildSummaries POST /api/admin/collective-insights/rebuild-summaries
// 全企業の行動サマリーをバッチ再集計する（管理画面用）
func (c *CollectiveInsightController) RebuildSummaries(w http.ResponseWriter, r *http.Request) {
	if err := c.svc.RebuildSummaries(); err != nil {
		writeInternalServerError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "rebuilt"})
}

// parseUserAndSession user_id・session_idをクエリから取得するヘルパー
func parseUserAndSession(r *http.Request) (uint, string, bool) {
	userIDStr := r.URL.Query().Get("user_id")
	sessionID := r.URL.Query().Get("session_id")
	if userIDStr == "" || sessionID == "" {
		return 0, "", false
	}
	uid, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		return 0, "", false
	}
	return uint(uid), sessionID, true
}
