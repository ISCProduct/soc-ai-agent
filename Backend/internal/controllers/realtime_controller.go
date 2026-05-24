package controllers

import (
	"Backend/internal/services"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type RealtimeController struct {
	interviewService     *services.InterviewService
	realtimeUsageService *services.RealtimeUsageService
}

func NewRealtimeController(interviewService *services.InterviewService, realtimeUsageService *services.RealtimeUsageService) *RealtimeController {
	return &RealtimeController{interviewService: interviewService, realtimeUsageService: realtimeUsageService}
}

type realtimeTokenRequest struct {
	UserID      uint `json:"user_id"`
	InterviewID uint `json:"interview_id"`
}

type realtimeTokenResponse struct {
	ClientSecret string `json:"client_secret"`
}

// Token POST /api/realtime/token
func (c *RealtimeController) Token(ctx echo.Context) error {
	var req realtimeTokenRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if req.UserID == 0 || req.InterviewID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "user_id and interview_id are required")
	}
	secret, err := c.interviewService.CreateRealtimeToken(ctx.Request().Context(), req.UserID, req.InterviewID)
	if err != nil {
		if err.Error() == "forbidden" {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		if strings.Contains(err.Error(), "realtime capacity exceeded") {
			return echo.NewHTTPError(http.StatusTooManyRequests, err.Error())
		}
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusOK, realtimeTokenResponse{ClientSecret: secret})
}

type sessionInfoResponse struct {
	SessionMinutes int `json:"session_minutes"`
}

// SessionInfo GET /api/realtime/session-info
// ユーザー向けのセッション時間（分）を返す。コスト情報は含まない。
func (c *RealtimeController) SessionInfo(ctx echo.Context) error {
	minutes := 10
	if c.realtimeUsageService != nil {
		minutes = c.realtimeUsageService.SessionDurationMinutes()
	}
	return ctx.JSON(http.StatusOK, sessionInfoResponse{SessionMinutes: minutes})
}
