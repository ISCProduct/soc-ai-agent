package controllers

import (
	"Backend/internal/config"
	"Backend/internal/middleware"
	"Backend/internal/services/schedule"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type GoogleCalendarController struct {
	calendarSync *schedule.CalendarSyncService
}

func NewGoogleCalendarController(calendarSync *schedule.CalendarSyncService) *GoogleCalendarController {
	return &GoogleCalendarController{calendarSync: calendarSync}
}

// ConnectStart GET /api/google-calendar/connect
// Googleカレンダー連携のOAuth認証を開始する（ユーザー認証必須）。
// Accept: application/json の場合は URL を JSON で返す（フロント側で window.location.href をセットするため）。
//
// FE(Next.jsプロキシ) ≠ Backend オリジンの本番では、Cookieに頼るstate検証は
// コールバック（Googleから直接Backendへ）まで届かず壊れるため、ユーザーIDと
// タイムスタンプを署名付きstateパラメータ自体に埋め込む（Cookie不要）。
func (c *GoogleCalendarController) ConnectStart(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	state, err := middleware.GenerateSignedState(fmt.Sprintf("%d", userID))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate state")
	}

	authURL := c.calendarSync.GetAuthURL(state)

	accept := ctx.Request().Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return ctx.JSON(http.StatusOK, map[string]string{"auth_url": authURL})
	}
	return ctx.Redirect(http.StatusTemporaryRedirect, authURL)
}

// ConnectCallback GET /api/google-calendar/callback
// GoogleカレンダーOAuthコールバック処理。
func (c *GoogleCalendarController) ConnectCallback(ctx echo.Context) error {
	userIDStr, ok := middleware.VerifySignedState(ctx.QueryParam("state"))
	if !ok {
		log.Printf("[GoogleCalendar] callback: invalid or expired state")
		return ctx.Redirect(http.StatusTemporaryRedirect, config.AppURL()+"/profile?calendar_error=auth_failed")
	}

	code := ctx.QueryParam("code")
	if code == "" {
		return ctx.Redirect(http.StatusTemporaryRedirect, config.AppURL()+"/profile?calendar_error=no_code")
	}

	uid64, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		return ctx.Redirect(http.StatusTemporaryRedirect, config.AppURL()+"/profile?calendar_error=invalid_user")
	}
	userID := uint(uid64)

	if err := c.calendarSync.ExchangeAndSave(ctx.Request().Context(), userID, code); err != nil {
		log.Printf("[GoogleCalendar] ExchangeAndSave error for user %d: %v", userID, err)
		return ctx.Redirect(http.StatusTemporaryRedirect, config.AppURL()+"/profile?calendar_error=save_failed")
	}

	return ctx.Redirect(http.StatusTemporaryRedirect, config.AppURL()+"/profile?calendar_connected=1")
}

// Status GET /api/google-calendar/status
// Googleカレンダー連携状態を返す（ユーザー認証必須）。
func (c *GoogleCalendarController) Status(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	connected := c.calendarSync.IsConnected(userID)
	return ctx.JSON(http.StatusOK, map[string]bool{"connected": connected})
}

// Disconnect DELETE /api/google-calendar/disconnect
// Googleカレンダー連携を解除する（ユーザー認証必須）。
func (c *GoogleCalendarController) Disconnect(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	if err := c.calendarSync.Disconnect(userID); err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]string{"status": "disconnected"})
}
