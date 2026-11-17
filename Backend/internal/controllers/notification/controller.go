// Package notification はアプリ内通知（#2033）の API。
//
// 宛先は必ず認証（EchoUserAuth）から決め、user_id をクエリや本文から受け取らない。
// 他人の通知は一覧に出ず、既読にしようとしても 404 になる。
package notification

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/models"
	notificationsvc "Backend/internal/services/notification"

	"github.com/labstack/echo/v4"
)

// Service はコントローラが使う通知の操作（notificationsvc.Service が満たす）。
type Service interface {
	List(userID uint, limit int, beforeID uint) ([]models.Notification, error)
	ListUnread(userID uint, limit int, excludeKinds []string) ([]models.Notification, error)
	UnreadCount(userID uint) (int64, error)
	MarkRead(userID, id uint) error
	MarkAllRead(userID uint) error
}

type Controller struct {
	svc Service
}

func NewController(svc Service) *Controller {
	return &Controller{svc: svc}
}

func currentUser(ctx echo.Context) (uint, error) {
	id, ok := middleware.UserIDFromContext(ctx.Request().Context())
	if !ok || id == 0 {
		return 0, echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	return id, nil
}

// List は本人の通知を新しい順に返す。GET /api/notifications?limit=&before_id=&unread=1&exclude_kind=guidance
// unread=1 のときは未読だけを返す（exclude_kind で種類を除ける。ホームのお知らせ用）。
func (c *Controller) List(ctx echo.Context) error {
	userID, err := currentUser(ctx)
	if err != nil {
		return err
	}
	limit := httpapi.LimitQuery(ctx, "limit", 20)
	var beforeID uint
	if raw := ctx.QueryParam("before_id"); raw != "" {
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "before_id が不正です")
		}
		beforeID = uint(v)
	}
	var rows []models.Notification
	if ctx.QueryParam("unread") == "1" {
		var exclude []string
		for _, k := range ctx.QueryParams()["exclude_kind"] {
			if k = strings.TrimSpace(k); k != "" {
				exclude = append(exclude, k)
			}
		}
		rows, err = c.svc.ListUnread(userID, limit, exclude)
	} else {
		rows, err = c.svc.List(userID, limit, beforeID)
	}
	if err != nil {
		return httpapi.InternalError(err)
	}
	unread, err := c.svc.UnreadCount(userID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	if rows == nil {
		rows = []models.Notification{}
	}
	return ctx.JSON(http.StatusOK, map[string]any{"notifications": rows, "unread_count": unread})
}

// UnreadCount は本人の未読の件数を返す（ヘッダのバッジ用）。GET /api/notifications/unread-count
func (c *Controller) UnreadCount(ctx echo.Context) error {
	userID, err := currentUser(ctx)
	if err != nil {
		return err
	}
	n, err := c.svc.UnreadCount(userID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"unread_count": n})
}

// MarkRead は本人の通知1件を既読にする。POST /api/notifications/:id/read
func (c *Controller) MarkRead(ctx echo.Context) error {
	userID, err := currentUser(ctx)
	if err != nil {
		return err
	}
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "id が不正です")
	}
	if err := c.svc.MarkRead(userID, uint(id)); err != nil {
		if errors.Is(err, notificationsvc.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "通知が見つかりません")
		}
		return httpapi.InternalError(err)
	}
	return ctx.NoContent(http.StatusNoContent)
}

// MarkAllRead は本人の未読をすべて既読にする。POST /api/notifications/read-all
func (c *Controller) MarkAllRead(ctx echo.Context) error {
	userID, err := currentUser(ctx)
	if err != nil {
		return err
	}
	if err := c.svc.MarkAllRead(userID); err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.NoContent(http.StatusNoContent)
}
