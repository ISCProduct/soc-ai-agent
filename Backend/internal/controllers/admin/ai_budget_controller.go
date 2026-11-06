package admin

import (
	"Backend/internal/controllers/httpapi"
	"Backend/internal/services/costs"
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// maxAIBudgetOffset は一覧の offset の上限。設定行は利用者と学校の数しかない。
const maxAIBudgetOffset = 100_000

type aiBudgetAuditLog interface {
	Record(actorEmail, action, targetType string, targetID uint, metadata map[string]any)
}

type AIBudgetController struct {
	service *costs.AIBudgetService
	audit   aiBudgetAuditLog
}

func NewAIBudgetController(service *costs.AIBudgetService, audit aiBudgetAuditLog) *AIBudgetController {
	return &AIBudgetController{service: service, audit: audit}
}

// aiBudgetErrorMessage は入力エラーの管理者向けの文を返す。
func aiBudgetErrorMessage(err error) string {
	if errors.Is(err, costs.ErrInvalidAIBudgetScope) {
		return "対象の種類が不正です"
	}
	return "対象または上限の値が不正です"
}

// List は GET /api/admin/ai-budgets の設定一覧を返す。
func (c *AIBudgetController) List(ctx echo.Context) error {
	limit := httpapi.LimitQuery(ctx, "limit", 50)
	offset := 0
	if raw := ctx.QueryParam("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 || parsed > maxAIBudgetOffset {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid offset")
		}
		offset = parsed
	}
	settings, total, err := c.service.ListSettingsWithUsage(ctx.Request().Context(), limit, offset)
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"settings": settings,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

type setAIBudgetRequest struct {
	Scope           string  `json:"scope"`
	ScopeID         uint    `json:"scope_id"`
	DailyLimitUSD   float64 `json:"daily_limit_usd"`
	MonthlyLimitUSD float64 `json:"monthly_limit_usd"`
}

// Set は PUT /api/admin/ai-budgets の上限を作成・更新する。
func (c *AIBudgetController) Set(ctx echo.Context) error {
	var req setAIBudgetRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	err := c.service.SetLimit(
		ctx.Request().Context(),
		req.Scope,
		req.ScopeID,
		req.DailyLimitUSD,
		req.MonthlyLimitUSD,
	)
	if errors.Is(err, costs.ErrInvalidAIBudgetScope) || errors.Is(err, costs.ErrInvalidAIBudgetLimit) {
		return echo.NewHTTPError(http.StatusBadRequest, aiBudgetErrorMessage(err))
	}
	if errors.Is(err, costs.ErrAIBudgetTargetNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "対象が見つかりません")
	}
	if err != nil {
		return httpapi.InternalError(err)
	}
	c.audit.Record(ctx.Request().Header.Get("X-Admin-Email"), "ai_budget.set", "ai_budget_"+req.Scope, req.ScopeID, map[string]any{
		"daily_limit_usd":   req.DailyLimitUSD,
		"monthly_limit_usd": req.MonthlyLimitUSD,
	})
	return ctx.JSON(http.StatusOK, map[string]any{"scope": req.Scope, "scope_id": req.ScopeID})
}

// Delete は DELETE /api/admin/ai-budgets/:scope/:scope_id の上限を解除する。
func (c *AIBudgetController) Delete(ctx echo.Context) error {
	scopeID, err := strconv.ParseUint(ctx.Param("scope_id"), 10, 64)
	if err != nil || scopeID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid scope_id")
	}
	err = c.service.DeleteLimit(ctx.Request().Context(), ctx.Param("scope"), uint(scopeID))
	if errors.Is(err, costs.ErrInvalidAIBudgetScope) || errors.Is(err, costs.ErrInvalidAIBudgetLimit) {
		return echo.NewHTTPError(http.StatusBadRequest, aiBudgetErrorMessage(err))
	}
	if err != nil {
		return httpapi.InternalError(err)
	}
	c.audit.Record(ctx.Request().Header.Get("X-Admin-Email"), "ai_budget.delete", "ai_budget_"+ctx.Param("scope"), uint(scopeID), nil)
	return ctx.NoContent(http.StatusNoContent)
}
