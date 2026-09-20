package controllers

import (
	"Backend/internal/controllers/httpapi"
	"Backend/internal/repositories"
	"Backend/internal/services/costs"
	"Backend/internal/services/interfaces"
	"net/http"
	"strings"
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
		return httpapi.InternalError(err)
	}
	realtimeMonthTotal := 0.0
	activeConnections := int64(0)
	realtimeUsers := []costs.RealtimeUserSummary{}
	if c.realtimeUsageService != nil {
		realtimeMonthTotal, err = c.realtimeUsageService.CurrentMonthTotalCost()
		if err != nil {
			return httpapi.InternalError(err)
		}
		activeConnections, err = c.realtimeUsageService.CurrentActiveCount()
		if err != nil {
			return httpapi.InternalError(err)
		}
		realtimeUsers, err = c.realtimeUsageService.GetUserBreakdown(30, 20)
		if err != nil {
			return httpapi.InternalError(err)
		}
	}

	since30d := time.Now().UTC().AddDate(0, 0, -30)
	modelBreakdown, err := c.costService.GetModelBreakdown(since30d)
	if err != nil {
		return httpapi.InternalError(err)
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

// Breakdown handles GET /api/admin/costs/breakdown?by=feature&days=30
//
// 機能別・プロバイダ別・モデル別・組織別の内訳を返す（#1294）。
// ローカル推論はコスト0で記録されるため、件数とレイテンシで「0円化できている量」を見る。
func (c *AdminCostsController) Breakdown(ctx echo.Context) error {
	dim := repositories.BreakdownDimension(strings.TrimSpace(ctx.QueryParam("by")))
	if dim == "" {
		dim = repositories.BreakdownByFeature
	}
	// クエリパラメータをそのまま SQL の列名にしないための検査。
	if !repositories.IsValidBreakdownDimension(dim) {
		return echo.NewHTTPError(http.StatusBadRequest, "by must be one of: feature, provider, model, organization")
	}

	days := httpapi.IntQuery(ctx, "days", 30)
	if days > 90 {
		days = 90
	}
	if days < 1 {
		days = 1
	}

	since := time.Now().UTC().AddDate(0, 0, -days)
	rows, err := c.costService.GetUsageBreakdown(ctx.Request().Context(), since, dim)
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"by":        string(dim),
		"days":      days,
		"breakdown": rows,
	})
}

// Daily handles GET /api/admin/costs/daily?days=30
func (c *AdminCostsController) Daily(ctx echo.Context) error {
	days := httpapi.IntQuery(ctx, "days", 30)
	if days > 90 {
		days = 90
	}
	rows, err := c.costService.GetDailyCosts(days)
	if err != nil {
		return httpapi.InternalError(err)
	}
	realtimeRows := []costs.RealtimeDailySummary{}
	if c.realtimeUsageService != nil {
		realtimeRows, err = c.realtimeUsageService.GetDailyUsage(days)
		if err != nil {
			return httpapi.InternalError(err)
		}
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"daily":          rows,
		"realtime_daily": realtimeRows,
	})
}

// Monthly handles GET /api/admin/costs/monthly?months=12
func (c *AdminCostsController) Monthly(ctx echo.Context) error {
	months := httpapi.IntQuery(ctx, "months", 12)
	if months > 24 {
		months = 24
	}
	rows, err := c.costService.GetMonthlyCosts(months)
	if err != nil {
		return httpapi.InternalError(err)
	}
	realtimeRows := []costs.RealtimeMonthlySummary{}
	if c.realtimeUsageService != nil {
		realtimeRows, err = c.realtimeUsageService.GetMonthlyUsage(months)
		if err != nil {
			return httpapi.InternalError(err)
		}
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"monthly":          rows,
		"realtime_monthly": realtimeRows,
	})
}
