package controllers_test

// EntitlementController.GetEntitlementsのHTTPハンドラーテスト(#985 CodeRabbit指摘)。
//
// 組織IDが解決できない、または組織取得に失敗した場合、entitlement.CurrentPlan()
// (DEFAULT_PLAN未設定時はPlanPro)へフォールバックしていると、一時的な障害で
// Free/期限切れ組織にもPro機能を返してしまう。fail-closed(PlanFree)であることを確認する。

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/services/organization"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
)

func withOrganizationID(r *http.Request, orgID uint) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.OrganizationIDContextKey, orgID)
	return r.WithContext(ctx)
}

func TestEntitlementController_GetEntitlements_NoOrgContext_FailsClosed(t *testing.T) {
	c := controllers.NewEntitlementController(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/entitlements", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, c.GetEntitlements, newCtx(req, rec), http.StatusOK)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "free", body["plan"])
}

func TestEntitlementController_GetEntitlements_OrgLookupError_FailsClosed(t *testing.T) {
	repo := &mocks.OrganizationRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(nil, errors.New("db error"))
	c := controllers.NewEntitlementController(organization.NewOrganizationService(repo))

	req := withOrganizationID(httptest.NewRequest(http.MethodGet, "/api/entitlements", nil), 1)
	rec := httptest.NewRecorder()
	assertStatus(t, c.GetEntitlements, newCtx(req, rec), http.StatusOK)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "free", body["plan"])
	features := body["features"].(map[string]any)
	assert.Equal(t, false, features["export"])
}

func TestEntitlementController_GetEntitlements_OrgResolved_UsesOrgPlan(t *testing.T) {
	repo := &mocks.OrganizationRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(&models.Organization{ID: 1, Plan: "pro"}, nil)
	c := controllers.NewEntitlementController(organization.NewOrganizationService(repo))

	req := withOrganizationID(httptest.NewRequest(http.MethodGet, "/api/entitlements", nil), 1)
	rec := httptest.NewRecorder()
	assertStatus(t, c.GetEntitlements, newCtx(req, rec), http.StatusOK)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "pro", body["plan"])
}
