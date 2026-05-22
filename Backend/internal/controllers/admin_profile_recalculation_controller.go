package controllers

import (
	"Backend/internal/services"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// AdminProfileRecalculationController 企業プロファイル再計算管理コントローラー
type AdminProfileRecalculationController struct {
	service *services.ProfileRecalculationService
}

func NewAdminProfileRecalculationController(service *services.ProfileRecalculationService) *AdminProfileRecalculationController {
	return &AdminProfileRecalculationController{service: service}
}

// RecalculateAll POST /api/admin/profile-recalculation
func (c *AdminProfileRecalculationController) RecalculateAll(ctx echo.Context) error {
	var req struct {
		MinSamples int `json:"min_samples"`
	}
	ctx.Bind(&req)

	results, err := c.service.RecalculateAll(req.MinSamples)
	if err != nil {
		return echoInternalError(err)
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

	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"results":       results,
		"total":         len(results),
		"updated_count": updated,
		"skipped_count": skipped,
	})
}

// RecalculateOne POST /api/admin/profile-recalculation/:company_id
func (c *AdminProfileRecalculationController) RecalculateOne(ctx echo.Context) error {
	var companyID uint
	if _, err := parseEchoUintParam(ctx, "company_id", &companyID); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company_id")
	}

	var req struct {
		MinSamples int `json:"min_samples"`
	}
	ctx.Bind(&req)

	result, err := c.service.RecalculateCompany(companyID, req.MinSamples)
	if err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, result)
}

// Rollback POST /api/admin/profile-recalculation/:company_id/rollback
func (c *AdminProfileRecalculationController) Rollback(ctx echo.Context) error {
	var companyID uint
	if _, err := parseEchoUintParam(ctx, "company_id", &companyID); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company_id")
	}

	if err := c.service.Rollback(companyID); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok":         true,
		"company_id": companyID,
		"message":    "プロファイルをロールバックしました",
	})
}

// GetHistory GET /api/admin/profile-recalculation/:company_id/history
func (c *AdminProfileRecalculationController) GetHistory(ctx echo.Context) error {
	var companyID uint
	if _, err := parseEchoUintParam(ctx, "company_id", &companyID); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company_id")
	}

	histories, err := c.service.GetHistory(companyID)
	if err != nil {
		return echoInternalError(err)
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

	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"histories": resp,
		"total":     len(resp),
	})
}

// parseEchoUintParam はパスパラメータを uint に変換するヘルパー。
func parseEchoUintParam(ctx echo.Context, name string, out *uint) (uint, error) {
	s := ctx.Param(name)
	id, err := strconv.ParseUint(s, 10, 64)
	if err != nil || id == 0 {
		return 0, err
	}
	*out = uint(id)
	return uint(id), nil
}
