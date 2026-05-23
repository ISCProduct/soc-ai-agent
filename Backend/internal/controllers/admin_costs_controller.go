package controllers

import (
	"Backend/internal/services"
	ifaces "Backend/internal/services/interfaces"
	"encoding/json"
	"net/http"
	"time"
)

type AdminCostsController struct {
	costService          ifaces.APICostService
	realtimeUsageService ifaces.RealtimeUsageService
}

func NewAdminCostsController(costService ifaces.APICostService, realtimeUsageService ifaces.RealtimeUsageService) *AdminCostsController {
	return &AdminCostsController{
		costService:          costService,
		realtimeUsageService: realtimeUsageService,
	}
}

// Summary handles GET /api/admin/costs/summary
func (c *AdminCostsController) Summary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	monthTotal, err := c.costService.GetCurrentMonthTotal()
	if err != nil {
		writeInternalServerError(w, err)
		return
	}
	realtimeMonthTotal := 0.0
	activeConnections := int64(0)
	realtimeUsers := []services.RealtimeUserSummary{}
	if c.realtimeUsageService != nil {
		realtimeMonthTotal, err = c.realtimeUsageService.CurrentMonthTotalCost()
		if err != nil {
			writeInternalServerError(w, err)
			return
		}
		activeConnections, err = c.realtimeUsageService.CurrentActiveCount()
		if err != nil {
			writeInternalServerError(w, err)
			return
		}
		realtimeUsers, err = c.realtimeUsageService.GetUserBreakdown(30, 20)
		if err != nil {
			writeInternalServerError(w, err)
			return
		}
	}

	since30d := time.Now().UTC().AddDate(0, 0, -30)
	modelBreakdown, err := c.costService.GetModelBreakdown(since30d)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"current_month_cost_usd": monthTotal,
		"model_breakdown":        modelBreakdown,
		"realtime": map[string]interface{}{
			"current_month_cost_usd": realtimeMonthTotal,
			"active_connections":     activeConnections,
			"user_breakdown":         realtimeUsers,
		},
	})
}

// Daily handles GET /api/admin/costs/daily?days=30
func (c *AdminCostsController) Daily(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	days := parseIntQuery(r, "days", 30)
	if days > 90 {
		days = 90
	}
	rows, err := c.costService.GetDailyCosts(days)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}
	realtimeRows := []services.RealtimeDailySummary{}
	if c.realtimeUsageService != nil {
		realtimeRows, err = c.realtimeUsageService.GetDailyUsage(days)
		if err != nil {
			writeInternalServerError(w, err)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"daily":          rows,
		"realtime_daily": realtimeRows,
	})
}

// Monthly handles GET /api/admin/costs/monthly?months=12
func (c *AdminCostsController) Monthly(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	months := parseIntQuery(r, "months", 12)
	if months > 24 {
		months = 24
	}
	rows, err := c.costService.GetMonthlyCosts(months)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}
	realtimeRows := []services.RealtimeMonthlySummary{}
	if c.realtimeUsageService != nil {
		realtimeRows, err = c.realtimeUsageService.GetMonthlyUsage(months)
		if err != nil {
			writeInternalServerError(w, err)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"monthly":          rows,
		"realtime_monthly": realtimeRows,
	})
}
