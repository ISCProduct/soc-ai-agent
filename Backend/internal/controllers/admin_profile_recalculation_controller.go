package controllers

import (
	"Backend/internal/services/interfaces"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// AdminProfileRecalculationController 企業プロファイル再計算管理コントローラー
type AdminProfileRecalculationController struct {
	service interfaces.ProfileRecalculationService
}

func NewAdminProfileRecalculationController(service interfaces.ProfileRecalculationService) *AdminProfileRecalculationController {
	return &AdminProfileRecalculationController{service: service}
}

// Route /api/admin/profile-recalculation 以下のルーティング
func (c *AdminProfileRecalculationController) Route(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path // 例: /api/admin/profile-recalculation/42/rollback

	// POST /api/admin/profile-recalculation → 全企業一括再計算
	if r.Method == http.MethodPost && path == "/api/admin/profile-recalculation" {
		c.RecalculateAll(w, r)
		return
	}

	// パスから company_id を取得
	// /api/admin/profile-recalculation/{company_id}
	// /api/admin/profile-recalculation/{company_id}/rollback
	// /api/admin/profile-recalculation/{company_id}/history
	trimmed := strings.TrimPrefix(path, "/api/admin/profile-recalculation/")
	parts := strings.SplitN(trimmed, "/", 2)
	companyID, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || companyID == 0 {
		http.Error(w, "Invalid company_id", http.StatusBadRequest)
		return
	}

	action := ""
	if len(parts) == 2 {
		action = parts[1]
	}

	switch {
	case r.Method == http.MethodPost && action == "rollback":
		c.Rollback(w, r, uint(companyID))
	case r.Method == http.MethodGet && action == "history":
		c.GetHistory(w, r, uint(companyID))
	case r.Method == http.MethodPost && action == "":
		c.RecalculateOne(w, r, uint(companyID))
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// RecalculateAll POST /api/admin/profile-recalculation
func (c *AdminProfileRecalculationController) RecalculateAll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MinSamples int `json:"min_samples"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	results, err := c.service.RecalculateAll(req.MinSamples)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}

	updated := 0
	skipped := 0
	for _, res := range results {
		if res.Updated {
			updated++
		} else {
			skipped++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results":       results,
		"total":         len(results),
		"updated_count": updated,
		"skipped_count": skipped,
	})
}

// RecalculateOne POST /api/admin/profile-recalculation/{company_id}
func (c *AdminProfileRecalculationController) RecalculateOne(w http.ResponseWriter, r *http.Request, companyID uint) {
	var req struct {
		MinSamples int `json:"min_samples"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	result, err := c.service.RecalculateCompany(companyID, req.MinSamples)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// Rollback POST /api/admin/profile-recalculation/{company_id}/rollback
func (c *AdminProfileRecalculationController) Rollback(w http.ResponseWriter, r *http.Request, companyID uint) {
	if err := c.service.Rollback(companyID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":         true,
		"company_id": companyID,
		"message":    "プロファイルをロールバックしました",
	})
}

// GetHistory GET /api/admin/profile-recalculation/{company_id}/history
func (c *AdminProfileRecalculationController) GetHistory(w http.ResponseWriter, r *http.Request, companyID uint) {
	histories, err := c.service.GetHistory(companyID)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}

	type HistoryResponse struct {
		ID          uint   `json:"id"`
		CompanyID   uint   `json:"company_id"`
		Trigger     string `json:"trigger"`
		SampleCount int    `json:"sample_count"`
		CreatedAt   string `json:"created_at"`
	}

	resp := make([]HistoryResponse, len(histories))
	for i, h := range histories {
		resp[i] = HistoryResponse{
			ID:          h.ID,
			CompanyID:   h.CompanyID,
			Trigger:     h.Trigger,
			SampleCount: h.SampleCount,
			CreatedAt:   h.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"histories": resp,
		"total":     len(resp),
	})
}
