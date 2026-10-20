package admin

import (
	"Backend/internal/controllers/httpapi"
	"Backend/internal/services/costs"
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type AIBudgetController struct {
	service *costs.AIBudgetService
}

func NewAIBudgetController(service *costs.AIBudgetService) *AIBudgetController {
	return &AIBudgetController{service: service}
}

// List は GET /api/admin/ai-budgets の設定一覧を返す。
func (c *AIBudgetController) List(ctx echo.Context) error {
	settings, err := c.service.ListSettings(ctx.Request().Context())
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"settings": settings})
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
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if errors.Is(err, costs.ErrAIBudgetTargetNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	if err != nil {
		return httpapi.InternalError(err)
	}
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
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.NoContent(http.StatusNoContent)
}
