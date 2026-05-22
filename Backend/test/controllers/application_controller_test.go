package controllers_test

// ApplicationControllerのHTTPハンドラーテスト (Issue #397)
//
// 実行: cd Backend && go test ./test/controllers/... -run Application -v

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	"Backend/internal/middleware"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// applicationControllerForTest はサービスなしで生成した ApplicationController
// バリデーションのみをテストするため、サービスを呼び出さないパスのみを対象とする
func newApplicationController() *controllers.ApplicationController {
	return controllers.NewApplicationController(nil)
}

// TestApplicationController_Apply_MethodNotAllowed はGETリクエストに405を返すことを検証
func TestApplicationController_Apply_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/applications", nil)
			w := httptest.NewRecorder()
			newApplicationController().Apply(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestApplicationController_Apply_InvalidBody はJSONパースエラーに400を返すことを検証
func TestApplicationController_Apply_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewBufferString("invalid json"))
	w := httptest.NewRecorder()
	newApplicationController().Apply(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestApplicationController_Apply_MissingFields は必須フィールド欠損時に400を返すことを検証
func TestApplicationController_Apply_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{"user_id=0", map[string]interface{}{"user_id": 0, "company_id": 1, "match_id": 1}},
		{"company_id=0", map[string]interface{}{"user_id": 1, "company_id": 0, "match_id": 1}},
		{"match_id=0", map[string]interface{}{"user_id": 1, "company_id": 1, "match_id": 0}},
		{"all zero", map[string]interface{}{"user_id": 0, "company_id": 0, "match_id": 0}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewBuffer(body))
			w := httptest.NewRecorder()
			newApplicationController().Apply(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// TestApplicationController_UpdateStatus_MethodNotAllowed はPUT以外に405を返すことを検証
func TestApplicationController_UpdateStatus_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPost, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/applications/1", nil)
			w := httptest.NewRecorder()
			newApplicationController().UpdateStatus(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestApplicationController_UpdateStatus_InvalidID はIDパースエラーに400を返すことを検証
func TestApplicationController_UpdateStatus_InvalidID(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"non-numeric", "/api/applications/abc"},
		{"zero", "/api/applications/0"},
		{"empty", "/api/applications/"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, tc.path, nil)
			w := httptest.NewRecorder()
			newApplicationController().UpdateStatus(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// TestApplicationController_UpdateStatus_InvalidBody はIDが正常でもJSONパースエラーに400を返すことを検証
func TestApplicationController_UpdateStatus_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/applications/1", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	newApplicationController().UpdateStatus(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestApplicationController_UpdateStatus_MissingFields はbody必須フィールド欠損に400を返すことを検証
func TestApplicationController_UpdateStatus_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{"user_id=0", map[string]interface{}{"user_id": 0, "status": "applied"}},
		{"status empty", map[string]interface{}{"user_id": 1, "status": ""}},
		{"both empty", map[string]interface{}{"user_id": 0, "status": ""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPut, "/api/applications/1", bytes.NewBuffer(body))
			w := httptest.NewRecorder()
			newApplicationController().UpdateStatus(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// TestApplicationController_List_MethodNotAllowed はGET以外に405を返すことを検証
func TestApplicationController_List_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/applications", nil)
			w := httptest.NewRecorder()
			newApplicationController().List(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestApplicationController_List_MissingUserID はuser_id未指定に400を返すことを検証
func TestApplicationController_List_MissingUserID(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		wantBad bool
	}{
		{"missing", "", true},
		{"non-numeric", "abc", true},
		{"zero", "0", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := "/api/applications"
			if tc.userID != "" {
				url += "?user_id=" + tc.userID
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			newApplicationController().List(w, req)
			if tc.wantBad {
				assert.Equal(t, http.StatusBadRequest, w.Code)
			}
		})
	}
}

// TestApplicationController_GetCorrelation_MethodNotAllowed はGET以外に405を返すことを検証
func TestApplicationController_GetCorrelation_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/applications/correlation", nil)
			w := httptest.NewRecorder()
			newApplicationController().GetCorrelation(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// withUserID はリクエストコンテキストにuserIDを設定するヘルパー
func withUserID(r *http.Request, userID uint) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey, userID)
	return r.WithContext(ctx)
}

// TestApplicationController_Apply_ValidInput は有効な入力でサービスが呼ばれることを検証（nilサービスへのアクセスをパニックで捕捉）
func TestApplicationController_Apply_ValidInput(t *testing.T) {
	body, _ := json.Marshal(map[string]interface{}{
		"user_id":    1,
		"company_id": 2,
		"match_id":   3,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	defer func() {
		r := recover()
		// nilサービスへのアクセスでpanicが発生する = バリデーションは通過している
		require.NotNil(t, r, "バリデーション通過後にサービスが呼ばれることを確認")
	}()
	newApplicationController().Apply(w, req)
}
