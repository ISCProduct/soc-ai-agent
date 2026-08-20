package controllers

import (
	"Backend/internal/entitlement"
	"Backend/internal/middleware"
	"Backend/internal/services/organization"
	"net/http"

	"github.com/labstack/echo/v4"
)

type EntitlementController struct {
	orgs *organization.OrganizationService
}

func NewEntitlementController(orgs *organization.OrganizationService) *EntitlementController {
	return &EntitlementController{orgs: orgs}
}

// GetEntitlements GET /api/entitlements — 現行プランの機能フラグ（#810。契約は #612）
//
// #985: 以前はグローバル環境変数DEFAULT_PLANのみを見ており、組織ごとに設定された
// plan/contract_end_date(admin organizations API経由)が反映されなかった。
// リクエストコンテキストの組織ID(EchoUserAuth経由)から実際の契約プランを解決する。
// 組織IDが解決できない/組織取得に失敗した場合はfail-closed(PlanFree)にする。
// CurrentPlan()(DEFAULT_PLAN未設定時はPlanPro)へフォールバックすると、
// 一時的な組織取得失敗でFree/期限切れ組織にもPro機能を返してしまう。
func (c *EntitlementController) GetEntitlements(ctx echo.Context) error {
	plan := entitlement.PlanFree
	if orgID, ok := middleware.OrganizationIDFromContext(ctx.Request().Context()); ok && c.orgs != nil {
		if org, err := c.orgs.Get(orgID); err == nil && org != nil {
			plan = entitlement.PlanForOrganization(org.Plan, org.ContractEndDate)
		}
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"plan":     plan,
		"features": entitlement.FeaturesFor(plan),
	})
}
