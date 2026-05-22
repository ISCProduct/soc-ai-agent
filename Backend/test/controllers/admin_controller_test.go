package controllers_test

// Admin系コントローラーのHTTPハンドラーテスト (Issue #397)
//
// 実行: cd Backend && go test ./test/controllers/... -run Admin -v

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"

	"github.com/stretchr/testify/assert"
)

// ---- AdminAuditController ----

func TestAdminAuditController_List_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminAuditController(nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/audit-logs", nil)
			w := httptest.NewRecorder()
			c.List(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// ---- AdminCostsController ----

func TestAdminCostsController_Summary_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminCostsController(nil, nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/costs/summary", nil)
			w := httptest.NewRecorder()
			c.Summary(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAdminCostsController_Daily_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminCostsController(nil, nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/costs/daily", nil)
			w := httptest.NewRecorder()
			c.Daily(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAdminCostsController_Monthly_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminCostsController(nil, nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/costs/monthly", nil)
			w := httptest.NewRecorder()
			c.Monthly(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// ---- AdminScoreValidationController ----

func TestAdminScoreValidationController_Route_NotFound(t *testing.T) {
	c := controllers.NewAdminScoreValidationController(nil)
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"POST correlation", http.MethodPost, "/api/admin/score-validation/correlation"},
		{"unknown path", http.MethodGet, "/api/admin/score-validation/unknown"},
		{"DELETE phase-metrics", http.MethodDelete, "/api/admin/score-validation/phase-metrics"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			c.Route(w, req)
			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

// Route経由でのメソッド不一致は404を返すことを検証
func TestAdminScoreValidationController_Route_MethodMismatch(t *testing.T) {
	c := controllers.NewAdminScoreValidationController(nil)
	// correlationはGETのみ許可 -> POSTは404になる
	req := httptest.NewRequest(http.MethodPost, "/api/admin/score-validation/correlation", nil)
	w := httptest.NewRecorder()
	c.Route(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---- AdminProfileRecalculationController ----

// TestAdminProfileRecalculationController_Route_NonPost はPOST以外のリクエストで
// 全企業一括再計算にルーティングされないことを検証（404か400が返る）
func TestAdminProfileRecalculationController_Route_NonPost(t *testing.T) {
	c := controllers.NewAdminProfileRecalculationController(nil)
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/profile-recalculation", nil)
			w := httptest.NewRecorder()
			c.Route(w, req)
			// POSTでないのでRecalculateAllには入らない（400か404）
			assert.NotEqual(t, http.StatusOK, w.Code)
			assert.NotEqual(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestAdminProfileRecalculationController_Route_InvalidCompanyID はcompany_id不正時に400を返すことを検証
func TestAdminProfileRecalculationController_Route_InvalidCompanyID(t *testing.T) {
	c := controllers.NewAdminProfileRecalculationController(nil)
	tests := []struct {
		name string
		path string
	}{
		{"non-numeric", "/api/admin/profile-recalculation/abc"},
		{"zero", "/api/admin/profile-recalculation/0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, nil)
			w := httptest.NewRecorder()
			c.Route(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// ---- AdminScraperSessionController ----

func TestAdminScraperSessionController_Sessions_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminScraperSessionController(nil)
	methods := []string{http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/scraper/sessions", nil)
			w := httptest.NewRecorder()
			c.Sessions(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAdminScraperSessionController_SessionDetail_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminScraperSessionController(nil)
	// DELETEのみ許可。GET/POST/PATCHは405を返す
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/scraper-sessions/site-key", nil)
			w := httptest.NewRecorder()
			c.SessionDetail(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestAdminScraperSessionController_SessionDetail_MissingKey はsite_key未指定に400を返すことを検証
func TestAdminScraperSessionController_SessionDetail_MissingKey(t *testing.T) {
	c := controllers.NewAdminScraperSessionController(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/scraper-sessions/", nil)
	w := httptest.NewRecorder()
	c.SessionDetail(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---- AdminDashboardController ----

func TestAdminDashboardController_ListUsers_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminDashboardController(nil, nil, nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/dashboard/users", nil)
			w := httptest.NewRecorder()
			c.ListUsers(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAdminDashboardController_UserSessions_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminDashboardController(nil, nil, nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/dashboard/users/1/sessions", nil)
			w := httptest.NewRecorder()
			c.UserSessions(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAdminDashboardController_ExportCSV_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminDashboardController(nil, nil, nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/dashboard/export/csv", nil)
			w := httptest.NewRecorder()
			c.ExportCSV(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// ---- AdminUserController ----

func TestAdminUserController_List_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminUserController(nil, nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/users", nil)
			w := httptest.NewRecorder()
			c.List(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// ---- AdminJobController ----

func TestAdminJobController_JobCategories_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminJobController(nil, nil, nil, nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/job-categories", nil)
			w := httptest.NewRecorder()
			c.JobCategories(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAdminJobController_JobPositions_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminJobController(nil, nil, nil, nil)
	methods := []string{http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/job-positions", nil)
			w := httptest.NewRecorder()
			c.JobPositions(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAdminJobController_GraduateEmployments_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminJobController(nil, nil, nil, nil)
	methods := []string{http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/graduate-employments", nil)
			w := httptest.NewRecorder()
			c.GraduateEmployments(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}
