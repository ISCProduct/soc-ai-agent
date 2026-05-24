package controllers

import (
	"Backend/internal/services/interfaces"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type AdminAuditController struct {
	service interfaces.AuditLogService
}

func NewAdminAuditController(service interfaces.AuditLogService) *AdminAuditController {
	return &AdminAuditController{service: service}
}

func (c *AdminAuditController) List(ctx echo.Context) error {
	limit := 50
	if value := ctx.QueryParam("limit"); value != "" {
		if v, err := strconv.Atoi(value); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}
	logs, err := c.service.List(limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch logs")
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"logs": logs,
	})
}
