package controllers

import (
	"Backend/internal/services"
	"Backend/internal/services/interfaces"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type AdminScraperSessionController struct {
	service interfaces.ScraperSessionService
}

func NewAdminScraperSessionController(service interfaces.ScraperSessionService) *AdminScraperSessionController {
	return &AdminScraperSessionController{service: service}
}

// List GET /api/admin/scraper-sessions
func (c *AdminScraperSessionController) List(ctx echo.Context) error {
	sessions, err := c.service.List()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch sessions")
	}
	return ctx.JSON(http.StatusOK, map[string]any{"sessions": sessions})
}

// Upsert POST /api/admin/scraper-sessions
func (c *AdminScraperSessionController) Upsert(ctx echo.Context) error {
	var req struct {
		SiteKey   string  `json:"site_key"`
		Cookies   string  `json:"cookies"`
		ExpiresAt *string `json:"expires_at,omitempty"`
	}
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	payload := services.ScraperSessionPayload{
		SiteKey: req.SiteKey,
		Cookies: req.Cookies,
	}
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid expires_at format (use RFC3339)")
		}
		payload.ExpiresAt = &t
	}

	session, err := c.service.Upsert(payload)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusOK, session)
}

// Delete DELETE /api/admin/scraper-sessions/:site_key
func (c *AdminScraperSessionController) Delete(ctx echo.Context) error {
	siteKey := ctx.Param("site_key")
	if siteKey == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "site_key is required")
	}
	if err := c.service.Delete(siteKey); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete session")
	}
	return ctx.NoContent(http.StatusNoContent)
}
