package routes_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	assessmentcontrollers "Backend/internal/controllers/assessment"
	"Backend/internal/routes"

	"github.com/labstack/echo/v4"
)

// 学校独自テンプレートの API は管理者認証の外では使えない（トークンなしは 401）。
// ルート定義から EchoAdminAuth が外れた場合に気づくための最低限の固定。
func TestSetupAssessmentSchoolRoutes_RequiresAdminAuth(t *testing.T) {
	e := echo.New()
	routes.SetupAssessmentSchoolRoutes(e.Group("/api"), &assessmentcontrollers.SchoolController{}, nil, nil, "secret")
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/admin/assessment-templates?school_id=1"},
		{http.MethodPost, "/api/admin/assessment-templates?school_id=1"},
		{http.MethodGet, "/api/admin/assessment-templates/1/students?school_id=1"},
	} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", tc.method, tc.path, rec.Code)
		}
	}
}
