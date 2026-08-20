package routes

import (
	"Backend/internal/controllers"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

// #985: entitlementsは組織ごとのプランを反映するため、EchoUserAuthで組織IDを解決してから渡す。
// /profileも以前は未認証でクエリのuser_idを信頼していたため、同様にEchoUserAuthを必須にする。
func SetupUserRoutes(api *echo.Group, profileController *controllers.IntegratedProfileController, entitlementController *controllers.EntitlementController, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver) {
	user := api.Group("/user", EchoUserAuth(userSecret, access, orgs))
	user.GET("/profile", profileController.GetProfile)
	api.GET("/entitlements", entitlementController.GetEntitlements, EchoUserAuth(userSecret, access, orgs))
}
