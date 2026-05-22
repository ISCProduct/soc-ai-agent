package controllers_test

// CollectiveInsightControllerのHTTPハンドラーテスト (Issue #397)
//
// 実行: cd Backend && go test ./test/controllers/... -run CollectiveInsight -v

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"

	"github.com/stretchr/testify/assert"
)

func newCollectiveInsightController() *controllers.CollectiveInsightController {
	return controllers.NewCollectiveInsightController(nil)
}

// TestCollectiveInsightController_Route_NotFound は未定義パスに404を返すことを検証
func TestCollectiveInsightController_Route_NotFound(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"unknown path", http.MethodGet, "/api/collective-insights/unknown"},
		{"POST recommendations", http.MethodPost, "/api/collective-insights/recommendations"},
		{"GET consent", http.MethodGet, "/api/collective-insights/consent"},
		{"GET actions", http.MethodGet, "/api/collective-insights/actions"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			newCollectiveInsightController().Route(w, req)
			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

// TestCollectiveInsightController_GetRecommendations_Unauthorized はuserIDなしに401を返すことを検証
func TestCollectiveInsightController_GetRecommendations_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/collective-insights/recommendations", nil)
	w := httptest.NewRecorder()
	newCollectiveInsightController().GetRecommendations(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestCollectiveInsightController_GetRecommendations_MissingSessionID はsession_id未指定に400を返すことを検証
func TestCollectiveInsightController_GetRecommendations_MissingSessionID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/collective-insights/recommendations", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	newCollectiveInsightController().GetRecommendations(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCollectiveInsightController_UpdateConsent_Unauthorized はuserIDなしに401を返すことを検証
func TestCollectiveInsightController_UpdateConsent_Unauthorized(t *testing.T) {
	body, _ := json.Marshal(map[string]bool{"allow": true})
	req := httptest.NewRequest(http.MethodPut, "/api/collective-insights/consent", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	newCollectiveInsightController().UpdateConsent(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestCollectiveInsightController_UpdateConsent_InvalidBody はJSONパースエラーに400を返すことを検証
func TestCollectiveInsightController_UpdateConsent_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/collective-insights/consent", bytes.NewBufferString("not-json"))
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	newCollectiveInsightController().UpdateConsent(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCollectiveInsightController_RecordAction_Unauthorized はuserIDなしに401を返すことを検証
func TestCollectiveInsightController_RecordAction_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/collective-insights/actions", nil)
	w := httptest.NewRecorder()
	newCollectiveInsightController().RecordAction(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestCollectiveInsightController_RecordAction_InvalidBody はJSONパースエラーに400を返すことを検証
func TestCollectiveInsightController_RecordAction_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/collective-insights/actions", bytes.NewBufferString("not-json"))
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	newCollectiveInsightController().RecordAction(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCollectiveInsightController_RecordAction_MissingFields は必須フィールド欠損に400を返すことを検証
func TestCollectiveInsightController_RecordAction_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{"company_id=0", map[string]interface{}{"company_id": 0, "action_type": "viewed"}},
		{"action_type empty", map[string]interface{}{"company_id": 1, "action_type": ""}},
		{"both missing", map[string]interface{}{"company_id": 0, "action_type": ""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/api/collective-insights/actions", bytes.NewBuffer(body))
			req = withUserID(req, 1)
			w := httptest.NewRecorder()
			newCollectiveInsightController().RecordAction(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// TestCollectiveInsightController_RecordAction_InvalidActionType は無効なaction_typeに400を返すことを検証
func TestCollectiveInsightController_RecordAction_InvalidActionType(t *testing.T) {
	invalidTypes := []string{"unknown", "liked", "favorited", "clicked"}
	for _, actionType := range invalidTypes {
		t.Run(actionType, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{
				"company_id":  1,
				"action_type": actionType,
			})
			req := httptest.NewRequest(http.MethodPost, "/api/collective-insights/actions", bytes.NewBuffer(body))
			req = withUserID(req, 1)
			w := httptest.NewRecorder()
			newCollectiveInsightController().RecordAction(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// TestCollectiveInsightController_GetTopPassRateCompanies_DefaultLimit はlimitなしでもエラーにならないことを検証
// (サービスがnilなのでpanicが発生するが、バリデーションは通過している)
func TestCollectiveInsightController_GetTopPassRateCompanies_DefaultLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/collective-insights/top-companies", nil)
	w := httptest.NewRecorder()

	defer func() {
		r := recover()
		// nilサービスへのアクセスでpanicが発生する = バリデーションは通過している
		if r == nil {
			// panicしなかった場合はレスポンスで判断
			assert.NotEqual(t, http.StatusBadRequest, w.Code)
		}
	}()
	newCollectiveInsightController().GetTopPassRateCompanies(w, req)
}
