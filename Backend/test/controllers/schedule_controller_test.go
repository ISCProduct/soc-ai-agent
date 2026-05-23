package controllers_test

// ScheduleControllerのHTTPハンドラーテスト (Issue #422)
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
)

func newScheduleController(svc *mocks.ScheduleServiceMock) *controllers.ScheduleController {
	return controllers.NewScheduleController(svc)
}

// ---- List ----

func TestScheduleController_List_MissingUserID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/schedule", nil)
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).List(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScheduleController_List_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/schedule?user_id=1", nil)
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).List(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestScheduleController_List_ServiceError(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("List", uint(1)).Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/schedule?user_id=1", nil)
	w := httptest.NewRecorder()
	newScheduleController(svc).List(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestScheduleController_List_Success(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	events := []models.ScheduleEvent{{UserID: 1, CompanyName: "Test Co"}}
	svc.On("List", uint(1)).Return(events, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/schedule?user_id=1", nil)
	w := httptest.NewRecorder()
	newScheduleController(svc).List(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ---- Create ----

func TestScheduleController_Create_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/schedule?user_id=1", nil)
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).Create(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestScheduleController_Create_MissingUserID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/schedule", nil)
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).Create(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScheduleController_Create_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/schedule?user_id=1", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).Create(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScheduleController_Create_InvalidScheduledAt(t *testing.T) {
	body, _ := json.Marshal(map[string]string{
		"company_name": "Test Co",
		"scheduled_at": "not-a-date",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/schedule?user_id=1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).Create(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
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
	req := httptest.NewRequest(http.MethodPost, "/api/schedule?user_id=1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newScheduleController(svc).Create(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
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
	req := httptest.NewRequest(http.MethodPost, "/api/schedule?user_id=1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newScheduleController(svc).Create(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

// ---- Get ----

func TestScheduleController_Get_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/schedule/1?user_id=1", nil)
	req.URL.Path = "/api/schedule/1"
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).Get(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestScheduleController_Get_MissingUserID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/schedule/1", nil)
	req.URL.Path = "/api/schedule/1"
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).Get(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScheduleController_Get_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/schedule/abc?user_id=1", nil)
	req.URL.Path = "/api/schedule/abc"
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).Get(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScheduleController_Get_Forbidden(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("Get", uint(1), uint(1)).Return(nil, errors.New("forbidden"))

	req := httptest.NewRequest(http.MethodGet, "/api/schedule/1?user_id=1", nil)
	req.URL.Path = "/api/schedule/1"
	w := httptest.NewRecorder()
	newScheduleController(svc).Get(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertExpectations(t)
}

func TestScheduleController_Get_NotFound(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("Get", uint(1), uint(1)).Return(nil, errors.New("not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/schedule/1?user_id=1", nil)
	req.URL.Path = "/api/schedule/1"
	w := httptest.NewRecorder()
	newScheduleController(svc).Get(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

func TestScheduleController_Get_Success(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	event := &models.ScheduleEvent{UserID: 1, CompanyName: "Test Co"}
	svc.On("Get", uint(1), uint(1)).Return(event, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/schedule/1?user_id=1", nil)
	req.URL.Path = "/api/schedule/1"
	w := httptest.NewRecorder()
	newScheduleController(svc).Get(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ---- Update ----

func TestScheduleController_Update_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/schedule/1?user_id=1", nil)
	req.URL.Path = "/api/schedule/1"
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).Update(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestScheduleController_Update_MissingUserID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/schedule/1", nil)
	req.URL.Path = "/api/schedule/1"
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).Update(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScheduleController_Update_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/schedule/abc?user_id=1", nil)
	req.URL.Path = "/api/schedule/abc"
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).Update(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScheduleController_Update_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/schedule/1?user_id=1", bytes.NewBufferString("not-json"))
	req.URL.Path = "/api/schedule/1"
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).Update(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScheduleController_Update_InvalidScheduledAt(t *testing.T) {
	body, _ := json.Marshal(map[string]string{
		"scheduled_at": "not-a-date",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/schedule/1?user_id=1", bytes.NewReader(body))
	req.URL.Path = "/api/schedule/1"
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).Update(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
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
	req := httptest.NewRequest(http.MethodPut, "/api/schedule/1?user_id=1", bytes.NewReader(body))
	req.URL.Path = "/api/schedule/1"
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newScheduleController(svc).Update(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
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
	req := httptest.NewRequest(http.MethodPut, "/api/schedule/1?user_id=1", bytes.NewReader(body))
	req.URL.Path = "/api/schedule/1"
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newScheduleController(svc).Update(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ---- Delete ----

func TestScheduleController_Delete_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/schedule/1?user_id=1", nil)
	req.URL.Path = "/api/schedule/1"
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).Delete(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestScheduleController_Delete_MissingUserID(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/schedule/1", nil)
	req.URL.Path = "/api/schedule/1"
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).Delete(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScheduleController_Delete_Forbidden(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("Delete", uint(1), uint(1)).Return(errors.New("forbidden"))

	req := httptest.NewRequest(http.MethodDelete, "/api/schedule/1?user_id=1", nil)
	req.URL.Path = "/api/schedule/1"
	w := httptest.NewRecorder()
	newScheduleController(svc).Delete(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertExpectations(t)
}

func TestScheduleController_Delete_NotFound(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("Delete", uint(1), uint(1)).Return(errors.New("not found"))

	req := httptest.NewRequest(http.MethodDelete, "/api/schedule/1?user_id=1", nil)
	req.URL.Path = "/api/schedule/1"
	w := httptest.NewRecorder()
	newScheduleController(svc).Delete(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

func TestScheduleController_Delete_Success(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("Delete", uint(1), uint(1)).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/schedule/1?user_id=1", nil)
	req.URL.Path = "/api/schedule/1"
	w := httptest.NewRecorder()
	newScheduleController(svc).Delete(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

// ---- ExportICS ----

func TestScheduleController_ExportICS_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/schedule/export/ics?user_id=1", nil)
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).ExportICS(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestScheduleController_ExportICS_MissingUserID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/schedule/export/ics", nil)
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).ExportICS(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScheduleController_ExportICS_ServiceError(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("ExportICS", uint(1)).Return("", errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/schedule/export/ics?user_id=1", nil)
	w := httptest.NewRecorder()
	newScheduleController(svc).ExportICS(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestScheduleController_ExportICS_Success(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("ExportICS", uint(1)).Return("BEGIN:VCALENDAR\nEND:VCALENDAR", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/schedule/export/ics?user_id=1", nil)
	w := httptest.NewRecorder()
	newScheduleController(svc).ExportICS(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/calendar")
	svc.AssertExpectations(t)
}

// ---- RouteList / RouteByID dispatch ----

func TestScheduleController_RouteList_Default(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/api/schedule?user_id=1", nil)
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).RouteList(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestScheduleController_RouteList_DispatchesGET(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("List", uint(1)).Return([]models.ScheduleEvent{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/schedule?user_id=1", nil)
	w := httptest.NewRecorder()
	newScheduleController(svc).RouteList(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestScheduleController_RouteList_DispatchesPOST(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	scheduledAt, _ := time.Parse(time.RFC3339, "2025-01-01T10:00:00Z")
	event := &models.ScheduleEvent{UserID: 1, CompanyName: "Test Co"}
	svc.On("Create", uint(1), "Test Co", "", "", scheduledAt, "").Return(event, nil)

	body, _ := json.Marshal(map[string]string{
		"company_name": "Test Co",
		"scheduled_at": "2025-01-01T10:00:00Z",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/schedule?user_id=1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newScheduleController(svc).RouteList(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestScheduleController_RouteByID_Default(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/api/schedule/1?user_id=1", nil)
	req.URL.Path = "/api/schedule/1"
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).RouteByID(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestScheduleController_RouteByID_DispatchesGET(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	event := &models.ScheduleEvent{UserID: 1, CompanyName: "Test Co"}
	svc.On("Get", uint(1), uint(1)).Return(event, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/schedule/1?user_id=1", nil)
	req.URL.Path = "/api/schedule/1"
	w := httptest.NewRecorder()
	newScheduleController(svc).RouteByID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestScheduleController_RouteByID_DispatchesDELETE(t *testing.T) {
	svc := &mocks.ScheduleServiceMock{}
	svc.On("Delete", uint(1), uint(1)).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/schedule/1?user_id=1", nil)
	req.URL.Path = "/api/schedule/1"
	w := httptest.NewRecorder()
	newScheduleController(svc).RouteByID(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

// ---- getUserID (補完) ----

func TestScheduleController_InvalidUserID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/schedule?user_id=abc", nil)
	w := httptest.NewRecorder()
	controllers.NewScheduleController(nil).List(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
