package notification

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"
	"Backend/internal/models"
	notificationsvc "Backend/internal/services/notification"

	"github.com/labstack/echo/v4"
)

// #2033: 通知の API は、宛先を認証から決める。

type fakeService struct {
	listUser uint
	readUser uint
	readID   uint
	readErr  error

	unreadExclude []string
}

func (f *fakeService) List(userID uint, limit int, beforeID uint) ([]models.Notification, error) {
	f.listUser = userID
	return nil, nil
}
func (f *fakeService) ListUnread(userID uint, limit int, excludeKinds []string) ([]models.Notification, error) {
	f.listUser = userID
	f.unreadExclude = excludeKinds
	return nil, nil
}
func (f *fakeService) UnreadCount(userID uint) (int64, error) { return 3, nil }
func (f *fakeService) MarkRead(userID, id uint) error {
	f.readUser, f.readID = userID, id
	return f.readErr
}
func (f *fakeService) MarkAllRead(userID uint) error { return nil }

func newCtx(method, target string, userID uint) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, target, nil)
	if userID > 0 {
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDContextKey, userID))
	}
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestList_クエリのuser_idは使わない(t *testing.T) {
	svc := &fakeService{}
	ctx, rec := newCtx(http.MethodGet, "/api/notifications?user_id=999", 7)
	if err := NewController(svc).List(ctx); err != nil {
		t.Fatalf("err = %v", err)
	}
	if svc.listUser != 7 {
		t.Fatalf("宛先 = %d, want 7（認証のユーザー）", svc.listUser)
	}
	if body := rec.Body.String(); body != "{\"notifications\":[],\"unread_count\":3}\n" {
		t.Fatalf("body = %s", body)
	}
}

func TestUnauthenticated(t *testing.T) {
	ctx, _ := newCtx(http.MethodGet, "/api/notifications", 0)
	var he *echo.HTTPError
	if err := NewController(&fakeService{}).List(ctx); !errors.As(err, &he) || he.Code != http.StatusUnauthorized {
		t.Fatalf("err = %v, want 401", err)
	}
}

func TestMarkRead(t *testing.T) {
	svc := &fakeService{}
	ctx, rec := newCtx(http.MethodPost, "/api/notifications/5/read", 7)
	ctx.SetParamNames("id")
	ctx.SetParamValues("5")
	if err := NewController(svc).MarkRead(ctx); err != nil || rec.Code != http.StatusNoContent {
		t.Fatalf("err = %v code = %d", err, rec.Code)
	}
	if svc.readUser != 7 || svc.readID != 5 {
		t.Fatalf("user=%d id=%d", svc.readUser, svc.readID)
	}

	// 他人の通知（サービスが ErrNotFound）は 404
	svc.readErr = notificationsvc.ErrNotFound
	ctx, _ = newCtx(http.MethodPost, "/api/notifications/6/read", 7)
	ctx.SetParamNames("id")
	ctx.SetParamValues("6")
	var he *echo.HTTPError
	if err := NewController(svc).MarkRead(ctx); !errors.As(err, &he) || he.Code != http.StatusNotFound {
		t.Fatalf("err = %v, want 404", err)
	}
}

func TestList_未読だけを種類を除いて返す(t *testing.T) {
	svc := &fakeService{}
	ctx, _ := newCtx(http.MethodGet, "/api/notifications?unread=1&exclude_kind=guidance", 7)
	if err := NewController(svc).List(ctx); err != nil {
		t.Fatalf("err = %v", err)
	}
	if svc.listUser != 7 || len(svc.unreadExclude) != 1 || svc.unreadExclude[0] != "guidance" {
		t.Fatalf("user=%d exclude=%v", svc.listUser, svc.unreadExclude)
	}
}
