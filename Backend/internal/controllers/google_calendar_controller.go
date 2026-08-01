package controllers

import (
	"Backend/internal/config"
	"Backend/internal/middleware"
	"Backend/internal/services"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

const gcalUserIDCookie = "gcal_uid"

type GoogleCalendarController struct {
	calendarSync *services.CalendarSyncService
}

func NewGoogleCalendarController(calendarSync *services.CalendarSyncService) *GoogleCalendarController {
	return &GoogleCalendarController{calendarSync: calendarSync}
}

// ConnectStart GET /api/google-calendar/connect
// Googleカレンダー連携のOAuth認証を開始する（ユーザー認証必須）。
// Accept: application/json の場合は URL を JSON で返す（フロント側で window.location.href をセットするため）。
func (c *GoogleCalendarController) ConnectStart(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	state, err := middleware.GenerateOAuthState(ctx.Response().Writer)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate state")
	}

	// userIDをHttpOnly Cookieに保存してコールバック時に取り出す（CSRF保護に加えてユーザー特定のため）
	secure := os.Getenv("APP_ENV") == "production"
	http.SetCookie(ctx.Response().Writer, &http.Cookie{
		Name:     gcalUserIDCookie,
		Value:    fmt.Sprintf("%d", userID),
		Path:     "/",
		MaxAge:   int((10 * time.Minute).Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

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
	if !middleware.VerifyOAuthState(ctx.Response().Writer, ctx.Request()) {
		log.Printf("[GoogleCalendar] callback: invalid or missing state")
		return ctx.Redirect(http.StatusTemporaryRedirect, config.AppURL()+"/profile?calendar_error=auth_failed")
	}

	code := ctx.QueryParam("code")
	if code == "" {
		return ctx.Redirect(http.StatusTemporaryRedirect, config.AppURL()+"/profile?calendar_error=no_code")
	}

	// gcal_uid CookieからユーザーIDを取り出す
	uidCookie, err := ctx.Request().Cookie(gcalUserIDCookie)
	if err != nil || uidCookie.Value == "" {
		log.Printf("[GoogleCalendar] callback: missing gcal_uid cookie")
		return ctx.Redirect(http.StatusTemporaryRedirect, config.AppURL()+"/profile?calendar_error=missing_user")
	}
	uid64, err := strconv.ParseUint(uidCookie.Value, 10, 64)
	if err != nil {
		return ctx.Redirect(http.StatusTemporaryRedirect, config.AppURL()+"/profile?calendar_error=invalid_user")
	}
	userID := uint(uid64)

	// Cookieを削除（使い捨て）
	secure := os.Getenv("APP_ENV") == "production"
	http.SetCookie(ctx.Response().Writer, &http.Cookie{
		Name:     gcalUserIDCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

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
