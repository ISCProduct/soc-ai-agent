package controllers

import (
	"Backend/internal/services"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type AdminCrawlController struct {
	service *services.CrawlService
	audit   *services.AuditLogService
}

func NewAdminCrawlController(service *services.CrawlService, audit *services.AuditLogService) *AdminCrawlController {
	return &AdminCrawlController{service: service, audit: audit}
}

// ListSources GET /api/admin/crawl-sources
func (c *AdminCrawlController) ListSources(ctx echo.Context) error {
	sources, err := c.service.ListSources()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch sources")
	}
	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"sources": sources,
	})
}

// CreateSource POST /api/admin/crawl-sources
func (c *AdminCrawlController) CreateSource(ctx echo.Context) error {
	var payload services.CrawlSourcePayload
	if err := ctx.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	source, err := c.service.CreateSource(payload)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "crawl_source.create", "crawl_source", source.ID, map[string]interface{}{
		"name": source.Name,
	})
	return ctx.JSON(http.StatusOK, source)
}

// UpdateSource PUT /api/admin/crawl-sources/:id
func (c *AdminCrawlController) UpdateSource(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid source id")
	}
	var payload services.CrawlSourcePayload
	if err := ctx.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	source, err := c.service.UpdateSource(uint(id), payload)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "crawl_source.update", "crawl_source", source.ID, map[string]interface{}{
		"name": source.Name,
	})
	return ctx.JSON(http.StatusOK, source)
}

// RunSource POST /api/admin/crawl-sources/:id/run
func (c *AdminCrawlController) RunSource(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid source id")
	}
	run, err := c.service.RunSource(uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "crawl_source.run", "crawl_source", uint(id), map[string]interface{}{
		"status": run.Status,
	})
	return ctx.JSON(http.StatusOK, run)
}

// Runs GET /api/admin/crawl-runs
func (c *AdminCrawlController) Runs(ctx echo.Context) error {
	var sourceID uint
	if value := ctx.QueryParam("source_id"); value != "" {
		if id, err := strconv.ParseUint(value, 10, 32); err == nil {
			sourceID = uint(id)
		}
	}
	runs, err := c.service.ListRuns(sourceID, 20)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch runs")
	}
	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"runs": runs,
	})
}
