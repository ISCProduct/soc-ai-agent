package controllers

import (
	"Backend/internal/services/interfaces"
	"encoding/json"
	"net/http"
	"strconv"
)

// ApplicationController 応募・選考ステータス管理コントローラー
type ApplicationController struct {
	appService interfaces.ApplicationService
}

func NewApplicationController(appService interfaces.ApplicationService) *ApplicationController {
	return &ApplicationController{appService: appService}
}

// Apply POST /api/applications - 企業への応募登録
func (c *ApplicationController) Apply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID    uint `json:"user_id"`
		CompanyID uint `json:"company_id"`
		MatchID   uint `json:"match_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserID == 0 || req.CompanyID == 0 || req.MatchID == 0 {
		http.Error(w, "user_id, company_id, match_id は必須です", http.StatusBadRequest)
		return
	}

	app, err := c.appService.Apply(req.UserID, req.CompanyID, req.MatchID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         app.ID,
		"user_id":    app.UserID,
		"company_id": app.CompanyID,
		"match_id":   app.MatchID,
		"status":     app.Status,
		"applied_at": app.AppliedAt,
	})
}

// UpdateStatus PUT /api/applications/{id} - 選考ステータス更新
func (c *ApplicationController) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// パスから ID を取得: /api/applications/123
	idStr := r.URL.Path[len("/api/applications/"):]
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		http.Error(w, "Invalid application ID", http.StatusBadRequest)
		return
	}

	var req struct {
		UserID uint   `json:"user_id"`
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserID == 0 || req.Status == "" {
		http.Error(w, "user_id と status は必須です", http.StatusBadRequest)
		return
	}

	app, err := c.appService.UpdateStatus(uint(id), req.UserID, req.Status, req.Notes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":     app.ID,
		"status": app.Status,
		"notes":  app.Notes,
	})
}

// List GET /api/applications?user_id=X - ユーザーの応募一覧取得
func (c *ApplicationController) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userIDStr := r.URL.Query().Get("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil || userID == 0 {
		http.Error(w, "user_id は必須です", http.StatusBadRequest)
		return
	}

	apps, err := c.appService.GetApplicationsByUser(uint(userID))
	if err != nil {
		http.Error(w, "データ取得エラー", http.StatusInternalServerError)
		return
	}

	type AppResponse struct {
		ID              uint        `json:"id"`
		CompanyID       uint        `json:"company_id"`
		CompanyName     string      `json:"company_name"`
		CompanyIndustry string      `json:"company_industry"`
		MatchID         uint        `json:"match_id"`
		Status          string      `json:"status"`
		Notes           string      `json:"notes"`
		AppliedAt       interface{} `json:"applied_at"`
		StatusUpdatedAt interface{} `json:"status_updated_at"`
	}

	resp := make([]AppResponse, len(apps))
	for i, app := range apps {
		name := ""
		industry := ""
		if app.Company != nil {
			name = app.Company.Name
			industry = app.Company.Industry
		}
		resp[i] = AppResponse{
			ID:              app.ID,
			CompanyID:       app.CompanyID,
			CompanyName:     name,
			CompanyIndustry: industry,
			MatchID:         app.MatchID,
			Status:          app.Status,
			Notes:           app.Notes,
			AppliedAt:       app.AppliedAt,
			StatusUpdatedAt: app.StatusUpdatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"applications": resp,
		"total":        len(resp),
	})
}

// GetCorrelation GET /api/applications/correlation?company_id=X - 相関分析データ取得
func (c *ApplicationController) GetCorrelation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	companyIDStr := r.URL.Query().Get("company_id")
	var companyID uint
	if companyIDStr != "" {
		id, err := strconv.ParseUint(companyIDStr, 10, 64)
		if err == nil {
			companyID = uint(id)
		}
	}

	data, err := c.appService.GetCorrelation(companyID)
	if err != nil {
		http.Error(w, "相関データ取得エラー", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"correlation": data,
		"total":       len(data),
	})
}
