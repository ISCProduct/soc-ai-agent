package controllers

import (
	"Backend/internal/services/costs"
	"Backend/internal/services/interfaces"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type AdminCostsController struct {
	costService          interfaces.APICostService
	realtimeUsageService interfaces.RealtimeUsageService
	searchBudgetService  interfaces.CompanySearchBudgetService
}

func NewAdminCostsController(costService interfaces.APICostService, realtimeUsageService interfaces.RealtimeUsageService, searchBudget ...interfaces.CompanySearchBudgetService) *AdminCostsController {
	ctrl := &AdminCostsController{
		costService:          costService,
		realtimeUsageService: realtimeUsageService,
	}
	if len(searchBudget) > 0 {
		ctrl.searchBudgetService = searchBudget[0]
	}
	return ctrl
}

// Summary handles GET /api/admin/costs/summary
func (c *AdminCostsController) Summary(ctx echo.Context) error {
	monthTotal, err := c.costService.GetCurrentMonthTotal()
	if err != nil {
		return echoInternalError(err)
	}
	realtimeMonthTotal := 0.0
	activeConnections := int64(0)
	realtimeUsers := []costs.RealtimeUserSummary{}
	if c.realtimeUsageService != nil {
		realtimeMonthTotal, err = c.realtimeUsageService.CurrentMonthTotalCost()
		if err != nil {
			return echoInternalError(err)
		}
		activeConnections, err = c.realtimeUsageService.CurrentActiveCount()
		if err != nil {
			return echoInternalError(err)
		}
		realtimeUsers, err = c.realtimeUsageService.GetUserBreakdown(30, 20)
		if err != nil {
			return echoInternalError(err)
		}
	}

	since30d := time.Now().UTC().AddDate(0, 0, -30)
	modelBreakdown, err := c.costService.GetModelBreakdown(since30d)
	if err != nil {
		return echoInternalError(err)
	}

	payload := map[string]any{
		"current_month_cost_usd": monthTotal,
		"model_breakdown":        modelBreakdown,
		"realtime": map[string]any{
			"current_month_cost_usd": realtimeMonthTotal,
			"active_connections":     activeConnections,
			"user_breakdown":         realtimeUsers,
		},
	}
	if c.searchBudgetService != nil {
		if status, serr := c.searchBudgetService.Status(); serr == nil {
			payload["company_search"] = status
		}
	}

	return ctx.JSON(http.StatusOK, payload)
}

// Daily handles GET /api/admin/costs/daily?days=30
func (c *AdminCostsController) Daily(ctx echo.Context) error {
	days := echoIntQuery(ctx, "days", 30)
	if days > 90 {
		days = 90
	}
	rows, err := c.costService.GetDailyCosts(days)
	if err != nil {
		return echoInternalError(err)
	}
	realtimeRows := []costs.RealtimeDailySummary{}
	if c.realtimeUsageService != nil {
		realtimeRows, err = c.realtimeUsageService.GetDailyUsage(days)
		if err != nil {
			return echoInternalError(err)
		}
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"daily":          rows,
		"realtime_daily": realtimeRows,
	})
}

// Monthly handles GET /api/admin/costs/monthly?months=12
func (c *AdminCostsController) Monthly(ctx echo.Context) error {
	months := echoIntQuery(ctx, "months", 12)
	if months > 24 {
		months = 24
	}
	rows, err := c.costService.GetMonthlyCosts(months)
	if err != nil {
		return echoInternalError(err)
	}
	realtimeRows := []costs.RealtimeMonthlySummary{}
	if c.realtimeUsageService != nil {
		realtimeRows, err = c.realtimeUsageService.GetMonthlyUsage(months)
		if err != nil {
			return echoInternalError(err)
		}
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"monthly":          rows,
		"realtime_monthly": realtimeRows,
	})
}
