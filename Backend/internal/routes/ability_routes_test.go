package routes_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	abilitycontrollers "Backend/internal/controllers/ability"
	"Backend/internal/routes"

	"github.com/labstack/echo/v4"
)

// 能力検査の API は本人認証の外では使えない（トークンなしは 401）。
// ルート定義から EchoUserAuth が外れると、他人の受検や問題の中身が無認証で読める。
func TestSetupAbilityRoutes_RequiresUserAuth(t *testing.T) {
	e := echo.New()
	routes.SetupAbilityRoutes(e.Group("/api"), &abilitycontrollers.Controller{}, "secret", nil, nil)
	const sid = "11111111-1111-1111-1111-111111111111"
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/ability-assessments"},
		{http.MethodPost, "/api/ability-assessments/1/sessions"},
		{http.MethodGet, "/api/ability-assessments/sessions/" + sid},
		{http.MethodPut, "/api/ability-assessments/sessions/" + sid + "/answers/1"},
		{http.MethodPost, "/api/ability-assessments/sessions/" + sid + "/submit"},
		{http.MethodGet, "/api/ability-assessments/sessions/" + sid + "/result"},
	} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", tc.method, tc.path, rec.Code)
		}
	}
}
