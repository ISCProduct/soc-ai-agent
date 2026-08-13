package controllers

import (
	"Backend/internal/entitlement"
	"net/http"

	"github.com/labstack/echo/v4"
)

// GetEntitlements GET /api/entitlements — 現行プランの機能フラグ（#810。契約は #612）
func GetEntitlements(ctx echo.Context) error {
	plan := entitlement.CurrentPlan()
	return ctx.JSON(http.StatusOK, map[string]any{
		"plan":     plan,
		"features": entitlement.FeaturesFor(plan),
	})
}
