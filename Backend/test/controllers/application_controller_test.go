package controllers_test

// ApplicationControllerのHTTPハンドラーテスト (Issue #397/#422)
//
// 実行: cd Backend && go test ./test/controllers/... -run Application -v

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"Backend/domain/entity"
	"Backend/internal/controllers"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


func newApplicationControllerWithMock(svc *mocks.ApplicationServiceMock) *controllers.ApplicationController {
	return controllers.NewApplicationController(svc)
}

// ---- Apply ----

func TestApplicationController_Apply_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/applications", nil)
			w := httptest.NewRecorder()
			controllers.NewApplicationController(nil).Apply(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestApplicationController_Apply_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewBufferString("invalid json"))
	w := httptest.NewRecorder()
	controllers.NewApplicationController(nil).Apply(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestApplicationController_Apply_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{"user_id=0", map[string]interface{}{"user_id": 0, "company_id": 1, "match_id": 1}},
		{"company_id=0", map[string]interface{}{"user_id": 1, "company_id": 0, "match_id": 1}},
		{"match_id=0", map[string]interface{}{"user_id": 1, "company_id": 1, "match_id": 0}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewBuffer(body))
			w := httptest.NewRecorder()
			controllers.NewApplicationController(nil).Apply(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestApplicationController_Apply_ServiceError(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	svc.On("Apply", uint(1), uint(2), uint(3)).Return(nil, errors.New("この企業にはすでに応募済みです"))

	body, _ := json.Marshal(map[string]interface{}{"user_id": 1, "company_id": 2, "match_id": 3})
	req := httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	newApplicationControllerWithMock(svc).Apply(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestApplicationController_Apply_Success(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	now := time.Now()
	app := &entity.UserApplicationStatus{
		UserID: 1, CompanyID: 2, MatchID: 3, Status: "applied", AppliedAt: &now,
	}
	svc.On("Apply", uint(1), uint(2), uint(3)).Return(app, nil)

	body, _ := json.Marshal(map[string]interface{}{"user_id": 1, "company_id": 2, "match_id": 3})
	req := httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	newApplicationControllerWithMock(svc).Apply(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "applied", resp["status"])
	svc.AssertExpectations(t)
}

// ---- UpdateStatus ----

func TestApplicationController_UpdateStatus_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/applications/1", nil)
			w := httptest.NewRecorder()
			controllers.NewApplicationController(nil).UpdateStatus(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestApplicationController_UpdateStatus_InvalidID(t *testing.T) {
	tests := []struct{ name, path string }{
		{"non-numeric", "/api/applications/abc"},
		{"zero", "/api/applications/0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, tc.path, nil)
			w := httptest.NewRecorder()
			controllers.NewApplicationController(nil).UpdateStatus(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestApplicationController_UpdateStatus_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{"user_id=0", map[string]interface{}{"user_id": 0, "status": "applied"}},
		{"status empty", map[string]interface{}{"user_id": 1, "status": ""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPut, "/api/applications/1", bytes.NewBuffer(body))
			w := httptest.NewRecorder()
			controllers.NewApplicationController(nil).UpdateStatus(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestApplicationController_UpdateStatus_ServiceError(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	svc.On("UpdateStatus", uint(1), uint(1), "applied", "").Return(nil, errors.New("権限がありません"))

	body, _ := json.Marshal(map[string]interface{}{"user_id": 1, "status": "applied"})
	req := httptest.NewRequest(http.MethodPut, "/api/applications/1", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	newApplicationControllerWithMock(svc).UpdateStatus(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestApplicationController_UpdateStatus_Success(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	app := &entity.UserApplicationStatus{Status: "interview", Notes: "通過"}
	svc.On("UpdateStatus", uint(1), uint(1), "interview", "通過").Return(app, nil)

	body, _ := json.Marshal(map[string]interface{}{"user_id": 1, "status": "interview", "notes": "通過"})
	req := httptest.NewRequest(http.MethodPut, "/api/applications/1", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	newApplicationControllerWithMock(svc).UpdateStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "interview", resp["status"])
	svc.AssertExpectations(t)
}

// ---- List ----

func TestApplicationController_List_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/applications", nil)
			w := httptest.NewRecorder()
			controllers.NewApplicationController(nil).List(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestApplicationController_List_MissingUserID(t *testing.T) {
	tests := []struct{ name, userID string }{
		{"missing", ""},
		{"non-numeric", "abc"},
		{"zero", "0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := "/api/applications"
			if tc.userID != "" {
				url += "?user_id=" + tc.userID
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			controllers.NewApplicationController(nil).List(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestApplicationController_List_ServiceError(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	svc.On("GetApplicationsByUser", uint(1)).Return(nil, errors.New("DB error"))

	req := httptest.NewRequest(http.MethodGet, "/api/applications?user_id=1", nil)
	w := httptest.NewRecorder()
	newApplicationControllerWithMock(svc).List(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestApplicationController_List_Success(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	apps := []*entity.UserApplicationStatus{
		{UserID: 1, CompanyID: 10, Status: "applied"},
		{UserID: 1, CompanyID: 20, Status: "interview"},
	}
	svc.On("GetApplicationsByUser", uint(1)).Return(apps, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/applications?user_id=1", nil)
	w := httptest.NewRecorder()
	newApplicationControllerWithMock(svc).List(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(2), resp["total"])
	svc.AssertExpectations(t)
}

// ---- GetCorrelation ----

func TestApplicationController_GetCorrelation_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/applications/correlation", nil)
			w := httptest.NewRecorder()
			controllers.NewApplicationController(nil).GetCorrelation(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestApplicationController_GetCorrelation_Success(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	data := []map[string]interface{}{{"company_id": 1, "pass_rate": 0.75}}
	svc.On("GetCorrelation", uint(0)).Return(data, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/applications/correlation", nil)
	w := httptest.NewRecorder()
	newApplicationControllerWithMock(svc).GetCorrelation(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(1), resp["total"])
	svc.AssertExpectations(t)
}

func TestApplicationController_GetCorrelation_WithCompanyID(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	data := []map[string]interface{}{{"company_id": 5, "pass_rate": 0.5}}
	svc.On("GetCorrelation", uint(5)).Return(data, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/applications/correlation?company_id=5", nil)
	w := httptest.NewRecorder()
	newApplicationControllerWithMock(svc).GetCorrelation(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}
