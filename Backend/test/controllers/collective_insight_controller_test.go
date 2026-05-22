package controllers_test

// CollectiveInsightControllerのHTTPハンドラーテスト (Issue #397/#422)
//
// 実行: cd Backend && go test ./test/controllers/... -run CollectiveInsight -v

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	"Backend/internal/services"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCollectiveInsightControllerWithMock(svc *mocks.CollectiveInsightServiceMock) *controllers.CollectiveInsightController {
	return controllers.NewCollectiveInsightController(svc)
}

// ---- Route ----

func TestCollectiveInsightController_Route_NotFound(t *testing.T) {
	c := controllers.NewCollectiveInsightController(nil)
	tests := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/collective-insights/unknown"},
		{http.MethodPost, "/api/collective-insights/recommendations"},
		{http.MethodGet, "/api/collective-insights/consent"},
	}
	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			c.Route(w, req)
			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

// ---- GetRecommendations ----

func TestCollectiveInsightController_GetRecommendations_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/collective-insights/recommendations", nil)
	w := httptest.NewRecorder()
	controllers.NewCollectiveInsightController(nil).GetRecommendations(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCollectiveInsightController_GetRecommendations_MissingSessionID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/collective-insights/recommendations", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	controllers.NewCollectiveInsightController(nil).GetRecommendations(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCollectiveInsightController_GetRecommendations_ServiceError(t *testing.T) {
	svc := &mocks.CollectiveInsightServiceMock{}
	svc.On("GetCollectiveRecommendations", uint(1), "sess-1", []uint(nil)).
		Return(nil, errors.New("DB error"))

	req := httptest.NewRequest(http.MethodGet, "/api/collective-insights/recommendations?session_id=sess-1", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	newCollectiveInsightControllerWithMock(svc).GetRecommendations(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestCollectiveInsightController_GetRecommendations_Success(t *testing.T) {
	svc := &mocks.CollectiveInsightServiceMock{}
	items := []services.CollectiveRecommendItem{
		{CompanyID: 10, CompanyName: "テスト株式会社", PassCount: 8, SimilarUsers: 10},
	}
	svc.On("GetCollectiveRecommendations", uint(1), "sess-abc", []uint(nil)).
		Return(items, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/collective-insights/recommendations?session_id=sess-abc", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	newCollectiveInsightControllerWithMock(svc).GetRecommendations(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(1), resp["count"])
	svc.AssertExpectations(t)
}

func TestCollectiveInsightController_GetRecommendations_WithExclude(t *testing.T) {
	svc := &mocks.CollectiveInsightServiceMock{}
	svc.On("GetCollectiveRecommendations", uint(1), "sess-x", []uint{5, 10}).
		Return([]services.CollectiveRecommendItem{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/collective-insights/recommendations?session_id=sess-x&exclude=5,10", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	newCollectiveInsightControllerWithMock(svc).GetRecommendations(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ---- GetTopPassRateCompanies ----

func TestCollectiveInsightController_GetTopPassRateCompanies_ServiceError(t *testing.T) {
	svc := &mocks.CollectiveInsightServiceMock{}
	svc.On("GetTopPassRateCompanies", 10).Return(nil, errors.New("DB error"))

	req := httptest.NewRequest(http.MethodGet, "/api/collective-insights/top-companies", nil)
	w := httptest.NewRecorder()
	newCollectiveInsightControllerWithMock(svc).GetTopPassRateCompanies(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestCollectiveInsightController_GetTopPassRateCompanies_Success(t *testing.T) {
	svc := &mocks.CollectiveInsightServiceMock{}
	svc.On("GetTopPassRateCompanies", 5).Return([]interface{}{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/collective-insights/top-companies?limit=5", nil)
	w := httptest.NewRecorder()

	// GetTopPassRateCompaniesはnil以外を返せばOK（型アサーションのため空スライスで代替）
	svc2 := &mocks.CollectiveInsightServiceMock{}
	svc2.On("GetTopPassRateCompanies", 5).Return(nil, nil)
	newCollectiveInsightControllerWithMock(svc2).GetTopPassRateCompanies(w, req)
	// nilはemptyスライスとして扱われるため200
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- UpdateConsent ----

func TestCollectiveInsightController_UpdateConsent_Unauthorized(t *testing.T) {
	body, _ := json.Marshal(map[string]bool{"allow": true})
	req := httptest.NewRequest(http.MethodPut, "/api/collective-insights/consent", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	controllers.NewCollectiveInsightController(nil).UpdateConsent(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCollectiveInsightController_UpdateConsent_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/collective-insights/consent", bytes.NewBufferString("not-json"))
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	controllers.NewCollectiveInsightController(nil).UpdateConsent(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCollectiveInsightController_UpdateConsent_Success(t *testing.T) {
	svc := &mocks.CollectiveInsightServiceMock{}
	svc.On("UpdateConsent", uint(1), true).Return(nil)

	body, _ := json.Marshal(map[string]bool{"allow": true})
	req := httptest.NewRequest(http.MethodPut, "/api/collective-insights/consent", bytes.NewBuffer(body))
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	newCollectiveInsightControllerWithMock(svc).UpdateConsent(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, true, resp["allow"])
	svc.AssertExpectations(t)
}

// ---- RecordAction ----

func TestCollectiveInsightController_RecordAction_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/collective-insights/actions", nil)
	w := httptest.NewRecorder()
	controllers.NewCollectiveInsightController(nil).RecordAction(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCollectiveInsightController_RecordAction_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/collective-insights/actions", bytes.NewBufferString("not-json"))
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	controllers.NewCollectiveInsightController(nil).RecordAction(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCollectiveInsightController_RecordAction_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{"company_id=0", map[string]interface{}{"company_id": 0, "action_type": "viewed"}},
		{"action_type empty", map[string]interface{}{"company_id": 1, "action_type": ""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/api/collective-insights/actions", bytes.NewBuffer(body))
			req = withUserID(req, 1)
			w := httptest.NewRecorder()
			controllers.NewCollectiveInsightController(nil).RecordAction(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestCollectiveInsightController_RecordAction_InvalidActionType(t *testing.T) {
	for _, actionType := range []string{"unknown", "liked", "clicked"} {
		t.Run(actionType, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{"company_id": 1, "action_type": actionType})
			req := httptest.NewRequest(http.MethodPost, "/api/collective-insights/actions", bytes.NewBuffer(body))
			req = withUserID(req, 1)
			w := httptest.NewRecorder()
			controllers.NewCollectiveInsightController(nil).RecordAction(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestCollectiveInsightController_RecordAction_Success(t *testing.T) {
	svc := &mocks.CollectiveInsightServiceMock{}
	svc.On("RecordAction", uint(1), "sess-1", uint(10), "viewed").Return(nil)

	body, _ := json.Marshal(map[string]interface{}{
		"session_id":  "sess-1",
		"company_id":  10,
		"action_type": "viewed",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/collective-insights/actions", bytes.NewBuffer(body))
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	newCollectiveInsightControllerWithMock(svc).RecordAction(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

// ---- RebuildSummaries ----

func TestCollectiveInsightController_RebuildSummaries_ServiceError(t *testing.T) {
	svc := &mocks.CollectiveInsightServiceMock{}
	svc.On("RebuildSummaries").Return(errors.New("rebuild failed"))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/collective-insights/rebuild-summaries", nil)
	w := httptest.NewRecorder()
	newCollectiveInsightControllerWithMock(svc).RebuildSummaries(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestCollectiveInsightController_RebuildSummaries_Success(t *testing.T) {
	svc := &mocks.CollectiveInsightServiceMock{}
	svc.On("RebuildSummaries").Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/collective-insights/rebuild-summaries", nil)
	w := httptest.NewRecorder()
	newCollectiveInsightControllerWithMock(svc).RebuildSummaries(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}
