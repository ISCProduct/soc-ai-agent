package controllers

import (
	"Backend/internal/services/interfaces"
	"encoding/json"
	"net/http"
	"strconv"
)

type AdminAuditController struct {
	service interfaces.AuditLogService
}

func NewAdminAuditController(service interfaces.AuditLogService) *AdminAuditController {
	return &AdminAuditController{service: service}
}

func (c *AdminAuditController) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 50
	if value := r.URL.Query().Get("limit"); value != "" {
		if v, err := strconv.Atoi(value); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}
	logs, err := c.service.List(limit)
	if err != nil {
		http.Error(w, "failed to fetch logs", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs": logs,
	})
}
