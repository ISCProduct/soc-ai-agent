package controllers

import (
	"Backend/internal/services/interfaces"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type ScheduleController struct {
	service interfaces.ScheduleService
}

func NewScheduleController(service interfaces.ScheduleService) *ScheduleController {
	return &ScheduleController{service: service}
}

type scheduleRequest struct {
	CompanyName string `json:"company_name"`
	Stage       string `json:"stage"`
	Title       string `json:"title"`
	ScheduledAt string `json:"scheduled_at"`
	Notes       string `json:"notes"`
}

// List GET /api/schedule
func (c *ScheduleController) List(ctx echo.Context) error {
	userID, err := echoScheduleUserID(ctx)
	if err != nil {
		return err
	}
	events, err := c.service.List(userID)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, events)
}

// Create POST /api/schedule
func (c *ScheduleController) Create(ctx echo.Context) error {
	userID, err := echoScheduleUserID(ctx)
	if err != nil {
		return err
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

// Get GET /api/schedule/:id
func (c *ScheduleController) Get(ctx echo.Context) error {
	userID, err := echoScheduleUserID(ctx)
	if err != nil {
		return err
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

// Update PUT /api/schedule/:id
func (c *ScheduleController) Update(ctx echo.Context) error {
	userID, err := echoScheduleUserID(ctx)
	if err != nil {
		return err
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

// Delete DELETE /api/schedule/:id
func (c *ScheduleController) Delete(ctx echo.Context) error {
	userID, err := echoScheduleUserID(ctx)
	if err != nil {
		return err
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

// ExportICS GET /api/schedule/export/ics
func (c *ScheduleController) ExportICS(ctx echo.Context) error {
	userID, err := echoScheduleUserID(ctx)
	if err != nil {
		return err
	}
	ics, err := c.service.ExportICS(userID)
	if err != nil {
		return echoInternalError(err)
	}
	ctx.Response().Header().Set("Content-Type", "text/calendar; charset=utf-8")
	ctx.Response().Header().Set("Content-Disposition", "attachment; filename=\"schedule.ics\"")
	return ctx.String(http.StatusOK, ics)
}

// echoScheduleUserID は認証済みユーザーID(JWT検証済み、EchoUserAuth経由)を取得する。
// クライアント指定のuser_idクエリパラメータは信用しない(#983: 以前はここが未検証で
// 任意ユーザーの予定にアクセスできた)。
func echoScheduleUserID(ctx echo.Context) (uint, error) {
	userID, ok := echoUserID(ctx)
	if !ok {
		return 0, echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	return userID, nil
}
