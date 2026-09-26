package routes

import (
	usercontrollers "Backend/internal/controllers/user"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

// #985: entitlementsは組織ごとのプランを反映するため、EchoUserAuthで組織IDを解決してから渡す。
// /profileも以前は未認証でクエリのuser_idを信頼していたため、同様にEchoUserAuthを必須にする。
func SetupUserRoutes(api *echo.Group, profileController *usercontrollers.IntegratedProfileController, entitlementController *usercontrollers.EntitlementController, preferenceController *usercontrollers.UserPreferenceController, guidanceController *usercontrollers.StudentGuidanceController, userSecret string, access auth.UserAccessGuard, orgs OrganizationIDResolver) {
	user := api.Group("/user", EchoUserAuth(userSecret, access, orgs))
	user.GET("/profile", profileController.GetProfile)
	// 学生本人の希望条件 (#1094)。企業向け学生検索のフィルタ軸になる。
	user.GET("/preferences", preferenceController.Get)
	user.PUT("/preferences", preferenceController.Put)
	user.GET("/industries", preferenceController.Industries)
	// 教員からの軌道修正・提案案内
	user.GET("/guidances", guidanceController.List)
	user.POST("/guidances/:id/dismiss", guidanceController.Dismiss)
	api.GET("/entitlements", entitlementController.GetEntitlements, EchoUserAuth(userSecret, access, orgs))
}
