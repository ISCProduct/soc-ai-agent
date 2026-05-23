package controllers_test

// Admin系コントローラーのHTTPハンドラーテスト
//
// 実行: cd Backend && go test ./test/controllers/... -run Admin -v

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
)

// ---- AdminAuditController ----

func TestAdminAuditController_New(t *testing.T) {
	c := controllers.NewAdminAuditController(nil)
	if c == nil {
		t.Fatal("NewAdminAuditController returned nil")
	}
}

// ---- AdminCostsController ----

func TestAdminCostsController_New(t *testing.T) {
	c := controllers.NewAdminCostsController(nil, nil)
	if c == nil {
		t.Fatal("NewAdminCostsController returned nil")
	}
}

// ---- AdminScoreValidationController ----

func TestAdminScoreValidationController_CreateVariant_InvalidBody(t *testing.T) {
	c := controllers.NewAdminScoreValidationController(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/score-validation/variants", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, c.CreateVariant, newCtx(req, rec), http.StatusBadRequest)
}

func TestAdminScoreValidationController_CreateVariant_MissingFields(t *testing.T) {
	c := controllers.NewAdminScoreValidationController(nil)
	body, _ := json.Marshal(map[string]any{"experiment_name": "", "variant_name": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/score-validation/variants", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, c.CreateVariant, newCtx(req, rec), http.StatusBadRequest)
}

// ---- AdminProfileRecalculationController ----

func TestAdminProfileRecalculationController_RecalculateOne_NonNumericID(t *testing.T) {
	c := controllers.NewAdminProfileRecalculationController(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/profile-recalculation/abc", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("company_id")
	ctx.SetParamValues("abc")
	assertStatus(t, c.RecalculateOne, ctx, http.StatusBadRequest)
}

// ---- AdminScraperSessionController ----

func TestAdminScraperSessionController_Delete_MissingKey(t *testing.T) {
	c := controllers.NewAdminScraperSessionController(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/scraper-sessions/", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("site_key")
	ctx.SetParamValues("")
	assertStatus(t, c.Delete, ctx, http.StatusBadRequest)
}

func TestAdminScraperSessionController_Upsert_InvalidBody(t *testing.T) {
	c := controllers.NewAdminScraperSessionController(nil)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/scraper-sessions", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, c.Upsert, newCtx(req, rec), http.StatusBadRequest)
}

func TestAdminScraperSessionController_Upsert_InvalidExpiresAt(t *testing.T) {
	c := controllers.NewAdminScraperSessionController(nil)
	expiresAt := "not-a-date"
	body, _ := json.Marshal(map[string]any{
		"site_key":   "test-site",
		"cookies":    "session=abc",
		"expires_at": expiresAt,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/scraper-sessions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, c.Upsert, newCtx(req, rec), http.StatusBadRequest)
}

// ---- AdminDashboardController ----

func TestAdminDashboardController_UserSessions_InvalidID(t *testing.T) {
	c := controllers.NewAdminDashboardController(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/users/abc/sessions", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	assertStatus(t, c.UserSessions, ctx, http.StatusBadRequest)
}

// ---- AdminUserController ----

func TestAdminUserController_Update_InvalidID(t *testing.T) {
	c := controllers.NewAdminUserController(nil, nil)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/abc", nil)
	rec := httptest.NewRecorder()
	ctx := newCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	assertStatus(t, c.Update, ctx, http.StatusBadRequest)
}

// ---- AdminJobController ----

func TestAdminJobController_New(t *testing.T) {
	c := controllers.NewAdminJobController(nil, nil, nil, nil)
	if c == nil {
		t.Fatal("NewAdminJobController returned nil")
	}
}
