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
	"Backend/internal/middleware"
	"Backend/internal/services/shared"
	"Backend/test/controllers/mocks"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// apiErrorCode はハンドラーの戻り値エラーから echo.HTTPError -> middleware.APIError の
// code フィールドを取り出すテストヘルパー（本番の CustomHTTPErrorHandler を経由せず直接検証する）。
func apiErrorCode(t *testing.T, err error) string {
	t.Helper()
	require.Error(t, err)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	apiErr, ok := he.Message.(middleware.APIError)
	require.True(t, ok, "expected middleware.APIError, got %T", he.Message)
	return apiErr.Code
}

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

// ---- AdminUpdateStatus ----

func TestApplicationController_AdminUpdateStatus_Success(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	app := &entity.UserApplicationStatus{Status: "document_screening"}
	// 管理者ルートは isAdmin=true 固定、userID は所有権チェック対象外のため 0 を渡す
	svc.On("UpdateStatus", uint(1), uint(0), "document_screening", "書類選考開始", true).Return(app, nil)

	body, _ := json.Marshal(map[string]any{"status": "document_screening", "notes": "書類選考開始"})
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/applications/1/status", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, newApplicationController(svc).AdminUpdateStatus, c, http.StatusOK)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "document_screening", resp["status"])
	svc.AssertExpectations(t)
}

func TestApplicationController_AdminUpdateStatus_MissingStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/applications/1/status", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, controllers.NewApplicationController(nil).AdminUpdateStatus, c, http.StatusBadRequest)
}

func TestApplicationController_AdminUpdateStatus_ClosedConflict_HasCode(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	svc.On("UpdateStatus", uint(1), uint(0), "document_screening", "", true).
		Return(nil, errors.New("application_already_closed: ステータス accepted は終了状態のため更新できません"))

	body, _ := json.Marshal(map[string]any{"status": "document_screening"})
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/applications/1/status", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	err := newApplicationController(svc).AdminUpdateStatus(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusConflict, he.Code)
	assert.Equal(t, "application_already_closed", apiErrorCode(t, err))
	svc.AssertExpectations(t)
}

func TestApplicationController_AdminUpdateStatus_InvalidTransition_HasCode(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	svc.On("UpdateStatus", uint(1), uint(0), "accepted", "", true).
		Return(nil, errors.New("invalid_status_transition: applied から accepted への遷移は許可されていません"))

	body, _ := json.Marshal(map[string]any{"status": "accepted"})
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/applications/1/status", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	err := newApplicationController(svc).AdminUpdateStatus(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusConflict, he.Code)
	assert.Equal(t, "invalid_status_transition", apiErrorCode(t, err))
	svc.AssertExpectations(t)
}

// ---- Withdraw ----

func TestApplicationController_Withdraw_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/applications/1/withdraw", nil)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, controllers.NewApplicationController(nil).Withdraw, c, http.StatusUnauthorized)
}

func TestApplicationController_Withdraw_Success(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	app := &entity.UserApplicationStatus{Status: "withdrawn"}
	svc.On("Withdraw", uint(1), uint(1), false).Return(app, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/applications/1/withdraw", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, newApplicationController(svc).Withdraw, c, http.StatusOK)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "withdrawn", resp["status"])
	svc.AssertExpectations(t)
}

func TestApplicationController_Withdraw_ClosedConflict_HasCode(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	svc.On("Withdraw", uint(1), uint(1), false).
		Return(nil, errors.New("application_already_closed: ステータス accepted は終了状態のため更新できません"))

	req := httptest.NewRequest(http.MethodPost, "/api/applications/1/withdraw", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	err := newApplicationController(svc).Withdraw(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusConflict, he.Code)
	assert.Equal(t, "application_already_closed", apiErrorCode(t, err))
	svc.AssertExpectations(t)
}

// ---- Accept ----

func TestApplicationController_Accept_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/applications/1/accept", nil)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, controllers.NewApplicationController(nil).Accept, c, http.StatusUnauthorized)
}

func TestApplicationController_Accept_Success(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	app := &entity.UserApplicationStatus{Status: "accepted"}
	svc.On("Accept", uint(1), uint(1), false).Return(app, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/applications/1/accept", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	assertStatus(t, newApplicationController(svc).Accept, c, http.StatusOK)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "accepted", resp["status"])
	svc.AssertExpectations(t)
}

func TestApplicationController_Accept_NotOffered_HasCode(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	svc.On("Accept", uint(1), uint(1), false).
		Return(nil, errors.New("application_not_offered: 内定状態でないため承諾できません（現在: applied）"))

	req := httptest.NewRequest(http.MethodPost, "/api/applications/1/accept", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	err := newApplicationController(svc).Accept(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusConflict, he.Code)
	assert.Equal(t, "application_not_offered", apiErrorCode(t, err))
	svc.AssertExpectations(t)
}

func TestApplicationController_Withdraw_Forbidden_HasCode(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	svc.On("Withdraw", uint(1), uint(1), false).
		Return(nil, errors.New("forbidden: 権限がありません"))

	req := httptest.NewRequest(http.MethodPost, "/api/applications/1/withdraw", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	err := newApplicationController(svc).Withdraw(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusForbidden, he.Code)
	assert.Equal(t, "forbidden", apiErrorCode(t, err))
	svc.AssertExpectations(t)
}

// ---- AdminList ----

func TestApplicationController_AdminList_Success(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	apps := []*entity.UserApplicationStatus{
		{UserID: 1, CompanyID: 2, Status: "document_screening"},
	}
	svc.On("ListForAdmin", uint(1), uint(2), "document_screening").Return(apps, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/applications?user_id=1&company_id=2&status=document_screening", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newApplicationController(svc).AdminList, newCtx(req, rec), http.StatusOK)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, float64(1), resp["total"])
	svc.AssertExpectations(t)
}

func TestApplicationController_AdminList_NoFilters(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	svc.On("ListForAdmin", uint(0), uint(0), "").Return([]*entity.UserApplicationStatus{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/applications", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newApplicationController(svc).AdminList, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestApplicationController_AdminList_InvalidUserID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/applications?user_id=abc", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewApplicationController(nil).AdminList, newCtx(req, rec), http.StatusBadRequest)
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
	svc.On("GetCorrelation", uint(1), uint(0)).Return(data, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/applications/correlation", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newApplicationController(svc).GetCorrelation, newCtx(req, rec), http.StatusOK)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, float64(1), resp["total"])
	svc.AssertExpectations(t)
}

func TestApplicationController_GetCorrelation_Forbidden(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	svc.On("GetCorrelation", uint(1), uint(99)).Return(nil, shared.ErrForbidden)

	req := httptest.NewRequest(http.MethodGet, "/api/applications/correlation?company_id=99", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newApplicationController(svc).GetCorrelation, newCtx(req, rec), http.StatusForbidden)
	svc.AssertExpectations(t)
}

func TestApplicationController_GetCorrelation_OwnerCompany(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	data := []map[string]any{{"match_score": 0.8, "status": "offered"}}
	svc.On("GetCorrelation", uint(1), uint(10)).Return(data, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/applications/correlation?company_id=10", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newApplicationController(svc).GetCorrelation, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

// ---- HR applications (#1083) ----

func TestApplicationController_HRList_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/hr/applications?company_id=10", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewApplicationController(nil).HRList, newCtx(req, rec), http.StatusUnauthorized)
}

func TestApplicationController_HRList_MissingCompanyID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/hr/applications", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewApplicationController(nil).HRList, newCtx(req, rec), http.StatusBadRequest)
}

func TestApplicationController_HRList_Forbidden(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	svc.On("ListForOwner", uint(1), uint(99), "").Return(nil, shared.ErrForbidden)
	req := httptest.NewRequest(http.MethodGet, "/api/hr/applications?company_id=99", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newApplicationController(svc).HRList, newCtx(req, rec), http.StatusForbidden)
	svc.AssertExpectations(t)
}

func TestApplicationController_HRUpdateStatus_Forbidden(t *testing.T) {
	svc := &mocks.ApplicationServiceMock{}
	svc.On("UpdateStatusAsOwner", uint(5), uint(1), "document_screening", "").Return(nil, shared.ErrForbidden)
	body, _ := json.Marshal(map[string]string{"status": "document_screening"})
	req := httptest.NewRequest(http.MethodPatch, "/api/hr/applications/5/status", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("5")
	assertStatus(t, newApplicationController(svc).HRUpdateStatus, c, http.StatusForbidden)
	svc.AssertExpectations(t)
}

func TestApplicationController_HRUpdateStatus_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/api/hr/applications/5/status", nil)
	rec := httptest.NewRecorder()
	c := newCtx(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("5")
	assertStatus(t, controllers.NewApplicationController(nil).HRUpdateStatus, c, http.StatusUnauthorized)
}
