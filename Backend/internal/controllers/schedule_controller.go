package controllers

import (
	"Backend/internal/services"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

type ScheduleController struct {
	service *services.ScheduleService
}

func NewScheduleController(service *services.ScheduleService) *ScheduleController {
	return &ScheduleController{service: service}
}

type scheduleRequest struct {
	CompanyName string `json:"company_name"`
	Stage       string `json:"stage"`
	Title       string `json:"title"`
	ScheduledAt string `json:"scheduled_at"`
	Notes       string `json:"notes"`
}

// List GET /api/schedule?user_id=X
func (c *ScheduleController) List(ctx echo.Context) error {
	userID, err := echoScheduleUserID(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	events, err := c.service.List(userID)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, events)
}

// Create POST /api/schedule?user_id=X
func (c *ScheduleController) Create(ctx echo.Context) error {
	userID, err := echoScheduleUserID(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	var req scheduleRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid scheduled_at format (RFC3339 expected)")
	}
	event, err := c.service.Create(userID, req.CompanyName, req.Stage, req.Title, scheduledAt, req.Notes)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusCreated, event)
}

// Get GET /api/schedule/:id?user_id=X
func (c *ScheduleController) Get(ctx echo.Context) error {
	userID, err := echoScheduleUserID(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	eventID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid event id")
	}
	event, err := c.service.Get(userID, eventID)
	if err != nil {
		if err.Error() == "forbidden" {
			return echo.NewHTTPError(http.StatusForbidden, "Forbidden")
		}
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	return ctx.JSON(http.StatusOK, event)
}

// Update PUT /api/schedule/:id?user_id=X
func (c *ScheduleController) Update(ctx echo.Context) error {
	userID, err := echoScheduleUserID(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	eventID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid event id")
	}
	var req scheduleRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	var scheduledAt time.Time
	if req.ScheduledAt != "" {
		scheduledAt, err = time.Parse(time.RFC3339, req.ScheduledAt)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid scheduled_at format (RFC3339 expected)")
		}
	}
	event, err := c.service.Update(userID, eventID, req.CompanyName, req.Stage, req.Title, scheduledAt, req.Notes)
	if err != nil {
		if err.Error() == "forbidden" {
			return echo.NewHTTPError(http.StatusForbidden, "Forbidden")
		}
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusOK, event)
}

// Delete DELETE /api/schedule/:id?user_id=X
func (c *ScheduleController) Delete(ctx echo.Context) error {
	userID, err := echoScheduleUserID(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	eventID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid event id")
	}
	if err := c.service.Delete(userID, eventID); err != nil {
		if err.Error() == "forbidden" {
			return echo.NewHTTPError(http.StatusForbidden, "Forbidden")
		}
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	return ctx.NoContent(http.StatusNoContent)
}

// ExportICS GET /api/schedule/export/ics?user_id=X
func (c *ScheduleController) ExportICS(ctx echo.Context) error {
	userID, err := echoScheduleUserID(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	ics, err := c.service.ExportICS(userID)
	if err != nil {
		return echoInternalError(err)
	}
	ctx.Response().Header().Set("Content-Type", "text/calendar; charset=utf-8")
	ctx.Response().Header().Set("Content-Disposition", "attachment; filename=\"schedule.ics\"")
	return ctx.String(http.StatusOK, ics)
}

// echoScheduleUserID はクエリパラメータ user_id をユーザーIDとして取得する。
func echoScheduleUserID(ctx echo.Context) (uint, error) {
	userIDStr := ctx.QueryParam("user_id")
	if userIDStr == "" {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "user_id is required")
	}
	id, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "invalid user_id")
	}
	return uint(id), nil
}
