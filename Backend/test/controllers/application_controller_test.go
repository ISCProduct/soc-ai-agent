package controllers_test

// ApplicationControllerのHTTPハンドラーテスト
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

func newApplicationController(svc *mocks.ApplicationServiceMock) *controllers.ApplicationController {
	return controllers.NewApplicationController(svc)
}

// ---- Apply ----

func TestApplicationController_Apply_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewBufferString(`{"company_id":1,"match_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewApplicationController(nil).Apply, newCtx(req, rec), http.StatusUnauthorized)
}

func TestApplicationController_Apply_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewApplicationController(nil).Apply, newCtx(req, rec), http.StatusBadRequest)
}

func TestApplicationController_Apply_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		body map[string]any
	}{
		{"company_id=0", map[string]any{"company_id": 0, "match_id": 1}},
		{"match_id=0", map[string]any{"company_id": 1, "match_id": 0}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req = withUserID(req, 1)
			rec := httptest.NewRecorder()
			assertStatus(t, controllers.NewApplicationController(nil).Apply, newCtx(req, rec), http.StatusBadRequest)
		})
	}
}

func TestApplicationController_Apply_ServiceError(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	svc.On("Apply", uint(1), uint(2), uint(3)).Return(nil, errors.New("already applied"))

	body, _ := json.Marshal(map[string]any{"user_id": 999, "company_id": 2, "match_id": 3})
	req := httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newApplicationController(svc).Apply, newCtx(req, rec), http.StatusBadRequest)
	svc.AssertExpectations(t)
}

func TestApplicationController_Apply_Success(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	now := time.Now()
	app := &entity.UserApplicationStatus{UserID: 1, CompanyID: 2, MatchID: 3, Status: "applied", AppliedAt: &now}
	svc.On("Apply", uint(1), uint(2), uint(3)).Return(app, nil)

	// body の user_id は無視され、認証ユーザーIDが使われる
	body, _ := json.Marshal(map[string]any{"user_id": 999, "company_id": 2, "match_id": 3})
	req := httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newApplicationController(svc).Apply, newCtx(req, rec), http.StatusCreated)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "applied", resp["status"])
	svc.AssertExpectations(t)
}

// ---- UpdateStatus ----

func TestApplicationController_UpdateStatus_Unauthorized(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"status": "interview"})
	req := httptest.NewRequest(http.MethodPut, "/api/applications/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, controllers.NewApplicationController(nil).UpdateStatus, c, http.StatusUnauthorized)
}

func TestApplicationController_UpdateStatus_InvalidID(t *testing.T) {
	tests := []struct{ name, id string }{
		{"non-numeric", "abc"},
		{"zero", "0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/applications/"+tc.id, nil)
			req = withUserID(req, 1)
			rec := httptest.NewRecorder()
			c := newCtx(req, rec)
			c.SetParamNames("id")
			c.SetParamValues(tc.id)
			assertStatus(t, controllers.NewApplicationController(nil).UpdateStatus, c, http.StatusBadRequest)
		})
	}
}

func TestApplicationController_UpdateStatus_MissingFields(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"status": ""})
	req := httptest.NewRequest(http.MethodPut, "/api/applications/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, controllers.NewApplicationController(nil).UpdateStatus, c, http.StatusBadRequest)
}

func TestApplicationController_UpdateStatus_Success(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	app := &entity.UserApplicationStatus{Status: "interview_in_progress", Notes: "通過"}
	svc.On("UpdateStatus", uint(1), uint(1), "interview_in_progress", "通過", false).Return(app, nil)

	body, _ := json.Marshal(map[string]any{"user_id": 999, "status": "interview_in_progress", "notes": "通過"})
	req := httptest.NewRequest(http.MethodPut, "/api/applications/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, newApplicationController(svc).UpdateStatus, c, http.StatusOK)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "interview_in_progress", resp["status"])
	svc.AssertExpectations(t)
}

func TestApplicationController_UpdateStatus_InvalidTransitionConflict(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	svc.On("UpdateStatus", uint(1), uint(1), "accepted", "", false).
		Return(nil, errors.New("invalid_status_transition: applied から accepted への遷移は許可されていません"))

	body, _ := json.Marshal(map[string]any{"status": "accepted"})
	req := httptest.NewRequest(http.MethodPut, "/api/applications/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, newApplicationController(svc).UpdateStatus, c, http.StatusConflict)
	svc.AssertExpectations(t)
}

func TestApplicationController_UpdateStatus_ClosedConflict(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	svc.On("UpdateStatus", uint(1), uint(1), "applied", "", false).
		Return(nil, errors.New("application_already_closed: ステータス accepted は終了状態のため更新できません"))

	body, _ := json.Marshal(map[string]any{"status": "applied"})
	req := httptest.NewRequest(http.MethodPut, "/api/applications/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, newApplicationController(svc).UpdateStatus, c, http.StatusConflict)
	svc.AssertExpectations(t)
}

func TestApplicationController_UpdateStatus_IgnoresClientIsAdmin(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	app := &entity.UserApplicationStatus{Status: "document_screening", Notes: "書類選考開始"}
	// is_admin=true を送ってもサービスには false が渡る
	svc.On("UpdateStatus", uint(1), uint(1), "document_screening", "書類選考開始", false).Return(app, nil)

	body, _ := json.Marshal(map[string]any{"user_id": 1, "status": "document_screening", "notes": "書類選考開始", "is_admin": true})
	req := httptest.NewRequest(http.MethodPut, "/api/applications/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, newApplicationController(svc).UpdateStatus, c, http.StatusOK)
	svc.AssertExpectations(t)
}

// ---- List ----

func TestApplicationController_List_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/applications", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewApplicationController(nil).List, newCtx(req, rec), http.StatusUnauthorized)
}

func TestApplicationController_List_ServiceError(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	svc.On("GetApplicationsByUser", uint(1)).Return(nil, errors.New("DB error"))

	req := httptest.NewRequest(http.MethodGet, "/api/applications", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newApplicationController(svc).List, newCtx(req, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

func TestApplicationController_List_Success(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	apps := []*entity.UserApplicationStatus{
		{UserID: 1, CompanyID: 10, Status: "applied"},
		{UserID: 1, CompanyID: 20, Status: "interview"},
	}
	svc.On("GetApplicationsByUser", uint(1)).Return(apps, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/applications?user_id=999", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newApplicationController(svc).List, newCtx(req, rec), http.StatusOK)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, float64(2), resp["total"])
	svc.AssertExpectations(t)
}

// ---- GetCorrelation ----

func TestApplicationController_GetCorrelation_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/applications/correlation", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewApplicationController(nil).GetCorrelation, newCtx(req, rec), http.StatusUnauthorized)
}

func TestApplicationController_GetCorrelation_Success(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	data := []map[string]any{{"company_id": 1, "pass_rate": 0.75}}
	svc.On("GetCorrelation", uint(0)).Return(data, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/applications/correlation", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newApplicationController(svc).GetCorrelation, newCtx(req, rec), http.StatusOK)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, float64(1), resp["total"])
	svc.AssertExpectations(t)
}
