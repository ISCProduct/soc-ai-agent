package controllers_test

// ScheduleControllerのHTTPハンドラーテスト (Issue #422)
//
// 実行: cd Backend && go test ./test/controllers/... -run Schedule -v
//
// #983: 認証済みユーザーIDはuser_idクエリパラメータではなくリクエストコンテキスト
// (EchoUserAuthが設定するmiddleware.UserIDContextKey)から取得するため、
// 各テストはwithUserIDでコンテキストにユーザーIDを注入する。

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
	"gorm.io/gorm"
)

func newScheduleController(svc *mocks.ScheduleServiceMock) *controllers.ScheduleController {
	return controllers.NewScheduleController(svc)
}

// ---- List ----

func TestScheduleController_List_ServiceError(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("List", uint(1)).Return(nil, errors.New("db error"))

	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/schedule", nil), 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newScheduleController(svc).List, newCtx(req, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

func TestScheduleController_List_Success(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	events := []models.ScheduleEvent{{UserID: 1, CompanyName: "Test Co"}}
	svc.On("List", uint(1)).Return(events, nil)

	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/schedule", nil), 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newScheduleController(svc).List, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

// ---- Create ----

func TestScheduleController_Create_InvalidBody(t *testing.T) {
	req := withUserID(httptest.NewRequest(http.MethodPost, "/api/schedule", bytes.NewBufferString("not-json")), 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewScheduleController(nil).Create, newCtx(req, rec), http.StatusBadRequest)
}

func TestScheduleController_Create_InvalidScheduledAt(t *testing.T) {
	body, _ := json.Marshal(map[string]string{
		"company_name": "Test Co",
		"scheduled_at": "not-a-date",
	})
	req := withUserID(httptest.NewRequest(http.MethodPost, "/api/schedule", bytes.NewReader(body)), 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewScheduleController(nil).Create, newCtx(req, rec), http.StatusBadRequest)
}

func TestScheduleController_Create_ServiceError(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	scheduledAt, _ := time.Parse(time.RFC3339, "2025-01-01T10:00:00Z")
	svc.On("Create", uint(1), "Test Co", "書類選考", "面接", scheduledAt, "").
		Return(nil, errors.New("company_name is required"))

	body, _ := json.Marshal(map[string]string{
		"company_name": "Test Co",
		"stage":        "書類選考",
		"title":        "面接",
		"scheduled_at": "2025-01-01T10:00:00Z",
	})
	req := withUserID(httptest.NewRequest(http.MethodPost, "/api/schedule", bytes.NewReader(body)), 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, newScheduleController(svc).Create, newCtx(req, rec), http.StatusBadRequest)
	svc.AssertExpectations(t)
}

func TestScheduleController_Create_Success(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	scheduledAt, _ := time.Parse(time.RFC3339, "2025-01-01T10:00:00Z")
	event := &models.ScheduleEvent{UserID: 1, CompanyName: "Test Co"}
	svc.On("Create", uint(1), "Test Co", "書類選考", "面接", scheduledAt, "").
		Return(event, nil)

	body, _ := json.Marshal(map[string]string{
		"company_name": "Test Co",
		"stage":        "書類選考",
		"title":        "面接",
		"scheduled_at": "2025-01-01T10:00:00Z",
	})
	req := withUserID(httptest.NewRequest(http.MethodPost, "/api/schedule", bytes.NewReader(body)), 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, newScheduleController(svc).Create, newCtx(req, rec), http.StatusCreated)
	svc.AssertExpectations(t)
}

// ---- Get ----

func TestScheduleController_Get_InvalidID(t *testing.T) {
	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/schedule/abc", nil), 1)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	assertStatus(t, controllers.NewScheduleController(nil).Get, ctx, http.StatusBadRequest)
}

func TestScheduleController_Get_Forbidden(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("Get", uint(1), uint(1)).Return(nil, errors.New("forbidden"))

	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/schedule/1", nil), 1)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newScheduleController(svc).Get, ctx, http.StatusForbidden)
	svc.AssertExpectations(t)
}

func TestScheduleController_Get_NotFound(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("Get", uint(1), uint(1)).Return(nil, errors.New("not found"))

	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/schedule/1", nil), 1)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newScheduleController(svc).Get, ctx, http.StatusNotFound)
	svc.AssertExpectations(t)
}

func TestScheduleController_Get_Success(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	event := &models.ScheduleEvent{UserID: 1, CompanyName: "Test Co"}
	svc.On("Get", uint(1), uint(1)).Return(event, nil)

	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/schedule/1", nil), 1)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newScheduleController(svc).Get, ctx, http.StatusOK)
	svc.AssertExpectations(t)
}

// ---- Update ----

func TestScheduleController_Update_InvalidID(t *testing.T) {
	req := withUserID(httptest.NewRequest(http.MethodPut, "/api/schedule/abc", nil), 1)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	assertStatus(t, controllers.NewScheduleController(nil).Update, ctx, http.StatusBadRequest)
}

func TestScheduleController_Update_InvalidBody(t *testing.T) {
	req := withUserID(httptest.NewRequest(http.MethodPut, "/api/schedule/1", bytes.NewBufferString("not-json")), 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, controllers.NewScheduleController(nil).Update, ctx, http.StatusBadRequest)
}

func TestScheduleController_Update_InvalidScheduledAt(t *testing.T) {
	body, _ := json.Marshal(map[string]string{
		"scheduled_at": "not-a-date",
	})
	req := withUserID(httptest.NewRequest(http.MethodPut, "/api/schedule/1", bytes.NewReader(body)), 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, controllers.NewScheduleController(nil).Update, ctx, http.StatusBadRequest)
}

func TestScheduleController_Update_Forbidden(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	scheduledAt, _ := time.Parse(time.RFC3339, "2025-01-01T10:00:00Z")
	svc.On("Update", uint(1), uint(1), "Test Co", "", "", scheduledAt, "").
		Return(nil, errors.New("forbidden"))

	body, _ := json.Marshal(map[string]string{
		"company_name": "Test Co",
		"scheduled_at": "2025-01-01T10:00:00Z",
	})
	req := withUserID(httptest.NewRequest(http.MethodPut, "/api/schedule/1", bytes.NewReader(body)), 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newScheduleController(svc).Update, ctx, http.StatusForbidden)
	svc.AssertExpectations(t)
}

func TestScheduleController_Update_NotFound(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	scheduledAt, _ := time.Parse(time.RFC3339, "2025-01-01T10:00:00Z")
	svc.On("Update", uint(1), uint(99), "Test Co", "", "", scheduledAt, "").
		Return(nil, gorm.ErrRecordNotFound)

	body, _ := json.Marshal(map[string]string{
		"company_name": "Test Co",
		"scheduled_at": "2025-01-01T10:00:00Z",
	})
	req := withUserID(httptest.NewRequest(http.MethodPut, "/api/schedule/99", bytes.NewReader(body)), 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("99")
	assertStatus(t, newScheduleController(svc).Update, ctx, http.StatusNotFound)
	svc.AssertExpectations(t)
}

func TestScheduleController_Update_Success(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	scheduledAt, _ := time.Parse(time.RFC3339, "2025-01-01T10:00:00Z")
	event := &models.ScheduleEvent{UserID: 1, CompanyName: "Test Co"}
	svc.On("Update", uint(1), uint(1), "Test Co", "", "", scheduledAt, "").
		Return(event, nil)

	body, _ := json.Marshal(map[string]string{
		"company_name": "Test Co",
		"scheduled_at": "2025-01-01T10:00:00Z",
	})
	req := withUserID(httptest.NewRequest(http.MethodPut, "/api/schedule/1", bytes.NewReader(body)), 1)
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

	req := withUserID(httptest.NewRequest(http.MethodDelete, "/api/schedule/1", nil), 1)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newScheduleController(svc).Delete, ctx, http.StatusForbidden)
	svc.AssertExpectations(t)
}

func TestScheduleController_Delete_NotFound(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("Delete", uint(1), uint(1)).Return(errors.New("not found"))

	req := withUserID(httptest.NewRequest(http.MethodDelete, "/api/schedule/1", nil), 1)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	assertStatus(t, newScheduleController(svc).Delete, ctx, http.StatusNotFound)
	svc.AssertExpectations(t)
}

func TestScheduleController_Delete_Success(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("Delete", uint(1), uint(1)).Return(nil)

	req := withUserID(httptest.NewRequest(http.MethodDelete, "/api/schedule/1", nil), 1)
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
	svc.On("ExportICS", uint(1)).Return("", errors.New("db error"))

	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/schedule/export/ics", nil), 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newScheduleController(svc).ExportICS, newCtx(req, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

func TestScheduleController_ExportICS_Success(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("ExportICS", uint(1)).Return("BEGIN:VCALENDAR\nEND:VCALENDAR", nil)

	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/schedule/export/ics", nil), 1)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	err := newScheduleController(svc).ExportICS(ctx)
	if err != nil {
		testEcho.HTTPErrorHandler(err, ctx)
	}
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/calendar")
	svc.AssertExpectations(t)
}
