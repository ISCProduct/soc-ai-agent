package controllers_test

// CollectiveInsightControllerのHTTPハンドラーテスト
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

func newCollectiveInsightController(svc *mocks.CollectiveInsightServiceMock) *controllers.CollectiveInsightController {
	return controllers.NewCollectiveInsightController(svc)
}

// ---- GetRecommendations ----

func TestCollectiveInsightController_GetRecommendations_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/collective-insights/recommendations", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewCollectiveInsightController(nil).GetRecommendations, newCtx(req, rec), http.StatusUnauthorized)
}

func TestCollectiveInsightController_GetRecommendations_MissingSessionID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/collective-insights/recommendations", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewCollectiveInsightController(nil).GetRecommendations, newCtx(req, rec), http.StatusBadRequest)
}

func TestCollectiveInsightController_GetRecommendations_ServiceError(t *testing.T) {
	svc := &mocks.CollectiveInsightServiceMock{}
	svc.On("GetCollectiveRecommendations", uint(1), "sess-1", []uint(nil)).
		Return(nil, errors.New("DB error"))

	req := httptest.NewRequest(http.MethodGet, "/api/collective-insights/recommendations?session_id=sess-1", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newCollectiveInsightController(svc).GetRecommendations, newCtx(req, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

func TestCollectiveInsightController_GetRecommendations_Success(t *testing.T) {
	svc := &mocks.CollectiveInsightServiceMock{}
	items := []services.CollectiveRecommendItem{
		{CompanyID: 10, CompanyName: "テスト株式会社", PassCount: 8, SimilarUsers: 10},
	}
	svc.On("GetCollectiveRecommendations", uint(1), "sess-abc", []uint(nil)).Return(items, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/collective-insights/recommendations?session_id=sess-abc", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newCollectiveInsightController(svc).GetRecommendations, newCtx(req, rec), http.StatusOK)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, float64(1), resp["count"])
	svc.AssertExpectations(t)
}

// ---- GetTopPassRateCompanies ----

func TestCollectiveInsightController_GetTopPassRateCompanies_ServiceError(t *testing.T) {
	svc := &mocks.CollectiveInsightServiceMock{}
	svc.On("GetTopPassRateCompanies", 10).Return(nil, errors.New("DB error"))

	req := httptest.NewRequest(http.MethodGet, "/api/collective-insights/top-companies", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newCollectiveInsightController(svc).GetTopPassRateCompanies, newCtx(req, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

func TestCollectiveInsightController_GetTopPassRateCompanies_Success(t *testing.T) {
	svc := &mocks.CollectiveInsightServiceMock{}
	svc.On("GetTopPassRateCompanies", 10).Return(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/collective-insights/top-companies", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newCollectiveInsightController(svc).GetTopPassRateCompanies, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

// ---- UpdateConsent ----

func TestCollectiveInsightController_UpdateConsent_Unauthorized(t *testing.T) {
	body, _ := json.Marshal(map[string]bool{"allow": true})
	req := httptest.NewRequest(http.MethodPut, "/api/collective-insights/consent", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewCollectiveInsightController(nil).UpdateConsent, newCtx(req, rec), http.StatusUnauthorized)
}

func TestCollectiveInsightController_UpdateConsent_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/collective-insights/consent", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewCollectiveInsightController(nil).UpdateConsent, newCtx(req, rec), http.StatusBadRequest)
}

func TestCollectiveInsightController_UpdateConsent_Success(t *testing.T) {
	svc := &mocks.CollectiveInsightServiceMock{}
	svc.On("UpdateConsent", uint(1), true).Return(nil)

	body, _ := json.Marshal(map[string]bool{"allow": true})
	req := httptest.NewRequest(http.MethodPut, "/api/collective-insights/consent", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newCollectiveInsightController(svc).UpdateConsent, newCtx(req, rec), http.StatusOK)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, true, resp["allow"])
	svc.AssertExpectations(t)
}

// ---- RecordAction ----

func TestCollectiveInsightController_RecordAction_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/collective-insights/actions", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewCollectiveInsightController(nil).RecordAction, newCtx(req, rec), http.StatusUnauthorized)
}

func TestCollectiveInsightController_RecordAction_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/collective-insights/actions", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewCollectiveInsightController(nil).RecordAction, newCtx(req, rec), http.StatusBadRequest)
}

func TestCollectiveInsightController_RecordAction_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		body map[string]any
	}{
		{"company_id=0", map[string]any{"company_id": 0, "action_type": "viewed"}},
		{"action_type empty", map[string]any{"company_id": 1, "action_type": ""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/api/collective-insights/actions", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req = withUserID(req, 1)
			rec := httptest.NewRecorder()
			assertStatus(t, controllers.NewCollectiveInsightController(nil).RecordAction, newCtx(req, rec), http.StatusBadRequest)
		})
	}
}

func TestCollectiveInsightController_RecordAction_InvalidActionType(t *testing.T) {
	for _, at := range []string{"unknown", "liked", "clicked"} {
		t.Run(at, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{"company_id": 1, "action_type": at})
			req := httptest.NewRequest(http.MethodPost, "/api/collective-insights/actions", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req = withUserID(req, 1)
			rec := httptest.NewRecorder()
			assertStatus(t, controllers.NewCollectiveInsightController(nil).RecordAction, newCtx(req, rec), http.StatusBadRequest)
		})
	}
}

func TestCollectiveInsightController_RecordAction_Success(t *testing.T) {
	svc := &mocks.CollectiveInsightServiceMock{}
	svc.On("RecordAction", uint(1), "sess-1", uint(10), "viewed").Return(nil)

	body, _ := json.Marshal(map[string]any{"session_id": "sess-1", "company_id": 10, "action_type": "viewed"})
	req := httptest.NewRequest(http.MethodPost, "/api/collective-insights/actions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newCollectiveInsightController(svc).RecordAction, newCtx(req, rec), http.StatusCreated)
	svc.AssertExpectations(t)
}

// ---- RebuildSummaries ----

func TestCollectiveInsightController_RebuildSummaries_ServiceError(t *testing.T) {
	svc := &mocks.CollectiveInsightServiceMock{}
	svc.On("RebuildSummaries").Return(errors.New("rebuild failed"))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/collective-insights/rebuild-summaries", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newCollectiveInsightController(svc).RebuildSummaries, newCtx(req, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

func TestCollectiveInsightController_RebuildSummaries_Success(t *testing.T) {
	svc := &mocks.CollectiveInsightServiceMock{}
	svc.On("RebuildSummaries").Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/collective-insights/rebuild-summaries", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newCollectiveInsightController(svc).RebuildSummaries, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}
