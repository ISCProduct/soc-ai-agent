package controllers_test

// ScheduleControllerのHTTPハンドラーテスト
//
// 実行: cd Backend && go test ./test/controllers/... -run Schedule -v

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"Backend/internal/controllers"
	"Backend/internal/models"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newScheduleController(svc *mocks.ScheduleServiceMock) *controllers.ScheduleController {
	return controllers.NewScheduleController(svc)
}

// ---- List ----

func TestScheduleController_List_Success(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	now := time.Now()
	events := []models.ScheduleEvent{
		{UserID: 1, Title: "一次面接", ScheduledAt: now},
		{UserID: 1, Title: "二次面接", ScheduledAt: now},
	}
	svc.On("List", uint(1)).Return(events, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/schedule?user_id=1", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newScheduleController(svc).List, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestScheduleController_List_ServiceError(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("List", uint(1)).Return(nil, errors.New("DB error"))

	req := httptest.NewRequest(http.MethodGet, "/api/schedule?user_id=1", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newScheduleController(svc).List, newCtx(req, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

// ---- Create ----

func TestScheduleController_Create_InvalidScheduledAt(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	body, _ := json.Marshal(map[string]any{
		"company_name": "テスト株式会社",
		"title":        "面接",
		"scheduled_at": "not-a-date",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/schedule?user_id=1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, c.Create, newCtx(req, rec), http.StatusBadRequest)
}

func TestScheduleController_Create_Success(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	scheduledAt := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	event := &models.ScheduleEvent{UserID: 1, Title: "一次面接", ScheduledAt: scheduledAt}
	svc.On("Create", uint(1), "テスト株式会社", "一次", "一次面接", scheduledAt, "備考").Return(event, nil)

	body, _ := json.Marshal(map[string]any{
		"company_name": "テスト株式会社",
		"stage":        "一次",
		"title":        "一次面接",
		"scheduled_at": "2026-06-01T10:00:00Z",
		"notes":        "備考",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/schedule?user_id=1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, newScheduleController(svc).Create, newCtx(req, rec), http.StatusCreated)
	svc.AssertExpectations(t)
}

func TestScheduleController_Create_ServiceError(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	scheduledAt := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	svc.On("Create", uint(1), "", "", "面接", scheduledAt, "").Return(nil, errors.New("validation error"))

	body, _ := json.Marshal(map[string]any{
		"title":        "面接",
		"scheduled_at": "2026-06-01T10:00:00Z",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/schedule?user_id=1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, newScheduleController(svc).Create, newCtx(req, rec), http.StatusBadRequest)
	svc.AssertExpectations(t)
}

// ---- Get ----

func TestScheduleController_Get_InvalidEventID(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/schedule/abc?user_id=1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	assertStatus(t, c.Get, ctx, http.StatusBadRequest)
}

func TestScheduleController_Get_Forbidden(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("Get", uint(1), uint(99)).Return(nil, errors.New("forbidden"))

	req := httptest.NewRequest(http.MethodGet, "/api/schedule/99?user_id=1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("99")
	assertStatus(t, newScheduleController(svc).Get, ctx, http.StatusForbidden)
	svc.AssertExpectations(t)
}

func TestScheduleController_Get_NotFound(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("Get", uint(1), uint(1)).Return(nil, errors.New("not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/schedule/1?user_id=1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newScheduleController(svc).Get, ctx, http.StatusNotFound)
	svc.AssertExpectations(t)
}

func TestScheduleController_Get_Success(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	event := &models.ScheduleEvent{ID: 1, UserID: 1, Title: "面接"}
	svc.On("Get", uint(1), uint(1)).Return(event, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/schedule/1?user_id=1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newScheduleController(svc).Get, ctx, http.StatusOK)
	svc.AssertExpectations(t)
}

// ---- Update ----

func TestScheduleController_Update_InvalidEventID(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	req := httptest.NewRequest(http.MethodPut, "/api/schedule/abc?user_id=1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	assertStatus(t, c.Update, ctx, http.StatusBadRequest)
}

func TestScheduleController_Update_InvalidScheduledAt(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	body, _ := json.Marshal(map[string]any{"scheduled_at": "invalid"})
	req := httptest.NewRequest(http.MethodPut, "/api/schedule/1?user_id=1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, c.Update, ctx, http.StatusBadRequest)
}

func TestScheduleController_Update_Forbidden(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("Update", uint(1), uint(1), "", "", "", time.Time{}, "").Return(nil, errors.New("forbidden"))

	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest(http.MethodPut, "/api/schedule/1?user_id=1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newScheduleController(svc).Update, ctx, http.StatusForbidden)
	svc.AssertExpectations(t)
}

func TestScheduleController_Update_Success(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	scheduledAt := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	event := &models.ScheduleEvent{ID: 1, UserID: 1, Title: "最終面接"}
	svc.On("Update", uint(1), uint(1), "株式会社A", "最終", "最終面接", scheduledAt, "").Return(event, nil)

	body, _ := json.Marshal(map[string]any{
		"company_name": "株式会社A",
		"stage":        "最終",
		"title":        "最終面接",
		"scheduled_at": "2026-07-01T09:00:00Z",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/schedule/1?user_id=1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newScheduleController(svc).Update, ctx, http.StatusOK)
	svc.AssertExpectations(t)
}

// ---- Delete ----

func TestScheduleController_Delete_Forbidden(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("Delete", uint(1), uint(1)).Return(errors.New("forbidden"))

	req := httptest.NewRequest(http.MethodDelete, "/api/schedule/1?user_id=1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newScheduleController(svc).Delete, ctx, http.StatusForbidden)
	svc.AssertExpectations(t)
}

func TestScheduleController_Delete_NotFound(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("Delete", uint(1), uint(99)).Return(errors.New("not found"))

	req := httptest.NewRequest(http.MethodDelete, "/api/schedule/99?user_id=1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("99")
	assertStatus(t, newScheduleController(svc).Delete, ctx, http.StatusNotFound)
	svc.AssertExpectations(t)
}

func TestScheduleController_Delete_Success(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("Delete", uint(1), uint(1)).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/schedule/1?user_id=1", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newScheduleController(svc).Delete, ctx, http.StatusNoContent)
	svc.AssertExpectations(t)
}

// ---- ExportICS ----

func TestScheduleController_ExportICS_ServiceError(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("ExportICS", uint(1)).Return("", errors.New("export failed"))

	req := httptest.NewRequest(http.MethodGet, "/api/schedule/export/ics?user_id=1", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newScheduleController(svc).ExportICS, newCtx(req, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

func TestScheduleController_ExportICS_Success(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("ExportICS", uint(1)).Return("BEGIN:VCALENDAR\nEND:VCALENDAR", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/schedule/export/ics?user_id=1", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newScheduleController(svc).ExportICS, newCtx(req, rec), http.StatusOK)

	assert.Equal(t, "text/calendar; charset=utf-8", rec.Header().Get("Content-Type"))
	svc.AssertExpectations(t)
}

// unused import guard
var _ = require.NoError
