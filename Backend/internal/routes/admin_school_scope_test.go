package routes_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/routes"
	"Backend/internal/services/school"

	"github.com/labstack/echo/v4"
)

// fakeSchoolRepo implements repository.SchoolRepository with in-memory data for tests.
type fakeSchoolRepo struct {
	assigned map[uint][]models.School
}

func (f *fakeSchoolRepo) Create(*models.School) error { return nil }
func (f *fakeSchoolRepo) Update(*models.School) error { return nil }
func (f *fakeSchoolRepo) FindByID(id uint) (*models.School, error) {
	return &models.School{ID: id}, nil
}
func (f *fakeSchoolRepo) FindByName(string) (*models.School, error)     { return nil, nil }
func (f *fakeSchoolRepo) List(int, int) ([]models.School, int64, error) { return nil, 0, nil }
func (f *fakeSchoolRepo) AddMember(*models.AdminSchoolMembership) error { return nil }
func (f *fakeSchoolRepo) RemoveMember(uint, uint) error                 { return nil }
func (f *fakeSchoolRepo) ListSchoolsForAdmin(userID uint) ([]models.School, error) {
	return f.assigned[userID], nil
}
func (f *fakeSchoolRepo) AddCompanyApproval(*models.SchoolCompanyApproval) error { return nil }
func (f *fakeSchoolRepo) RemoveCompanyApproval(uint, uint) error                 { return nil }
func (f *fakeSchoolRepo) IsCompanyApproved(uint, uint) (bool, error)             { return false, nil }
func (f *fakeSchoolRepo) ListApprovedCompanyIDs(uint) ([]uint, error)            { return nil, nil }

func newSchoolScopeTestEcho(schools *school.SchoolService, adminUserID uint) (*echo.Echo, *httptest.ResponseRecorder) {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := context.WithValue(c.Request().Context(), middleware.AdminUserIDContextKey, adminUserID)
			ctx = context.WithValue(ctx, middleware.AdminIsPlatformContextKey, true)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})
	e.GET("/x", func(c echo.Context) error { return c.String(http.StatusOK, "ok") }, routes.EchoAdminSchoolScope(schools))
	return e, httptest.NewRecorder()
}

func TestEchoAdminSchoolScope_UnrestrictedNoFilter(t *testing.T) {
	schools := school.NewSchoolService(&fakeSchoolRepo{assigned: map[uint][]models.School{}})
	e, rec := newSchoolScopeTestEcho(schools, 1)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestEchoAdminSchoolScope_RestrictedMissingSchoolID(t *testing.T) {
	schools := school.NewSchoolService(&fakeSchoolRepo{assigned: map[uint][]models.School{
		2: {{ID: 5}},
	}})
	e, rec := newSchoolScopeTestEcho(schools, 2)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestEchoAdminSchoolScope_RestrictedDeniedSchool(t *testing.T) {
	schools := school.NewSchoolService(&fakeSchoolRepo{assigned: map[uint][]models.School{
		2: {{ID: 5}},
	}})
	e, rec := newSchoolScopeTestEcho(schools, 2)

	req := httptest.NewRequest(http.MethodGet, "/x?school_id=6", nil)
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestEchoAdminSchoolScope_RestrictedAllowedSchool(t *testing.T) {
	schools := school.NewSchoolService(&fakeSchoolRepo{assigned: map[uint][]models.School{
		2: {{ID: 5}},
	}})
	e, rec := newSchoolScopeTestEcho(schools, 2)

	req := httptest.NewRequest(http.MethodGet, "/x?school_id=5", nil)
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestEchoRequirePlatformAdmin_UnrestrictedOK(t *testing.T) {
	schools := school.NewSchoolService(&fakeSchoolRepo{assigned: map[uint][]models.School{}})
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := context.WithValue(c.Request().Context(), middleware.AdminUserIDContextKey, uint(1))
			ctx = context.WithValue(ctx, middleware.AdminIsPlatformContextKey, true)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})
	e.GET("/x", func(c echo.Context) error { return c.String(http.StatusOK, "ok") }, routes.EchoRequirePlatformAdmin(schools))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestEchoRequirePlatformAdmin_RestrictedForbidden(t *testing.T) {
	schools := school.NewSchoolService(&fakeSchoolRepo{assigned: map[uint][]models.School{
		2: {{ID: 5}},
	}})
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := context.WithValue(c.Request().Context(), middleware.AdminUserIDContextKey, uint(2))
			ctx = context.WithValue(ctx, middleware.AdminIsPlatformContextKey, true)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})
	e.GET("/x", func(c echo.Context) error { return c.String(http.StatusOK, "ok") }, routes.EchoRequirePlatformAdmin(schools))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// ── 職員（role=staff, is_admin=false）の認可 ──────────────────────────

// stashPrincipal は userID と is_platform を積む簡易ミドルウェア。
func stashPrincipal(e *echo.Echo, userID uint, isPlatform bool) {
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := context.WithValue(c.Request().Context(), middleware.AdminUserIDContextKey, userID)
			ctx = context.WithValue(ctx, middleware.AdminIsPlatformContextKey, isPlatform)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})
}

// 職員は担当校に限って教員ルートへ入れる。
func TestSchoolScope_StaffAllowedOwnSchool(t *testing.T) {
	schools := school.NewSchoolService(&fakeSchoolRepo{assigned: map[uint][]models.School{3: {{ID: 5}}}})
	e := echo.New()
	stashPrincipal(e, 3, false)
	e.GET("/x", func(c echo.Context) error { return c.String(http.StatusOK, "ok") }, routes.EchoAdminSchoolScope(schools))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x?school_id=5", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("staff should access own school; status=%d", rec.Code)
	}
}

// 職員が担当校0件だと「無制限」に化けず、school_id を付けても他校は弾かれる。
func TestSchoolScope_StaffNoMembershipDenied(t *testing.T) {
	schools := school.NewSchoolService(&fakeSchoolRepo{assigned: map[uint][]models.School{}})
	e := echo.New()
	stashPrincipal(e, 7, false)
	e.GET("/x", func(c echo.Context) error { return c.String(http.StatusOK, "ok") }, routes.EchoAdminSchoolScope(schools))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x?school_id=5", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("staff with no memberships must be denied; status=%d", rec.Code)
	}
}

// 職員はプラットフォーム系ルートに入れない（担当校0件でも）。
func TestPlatformAdmin_StaffForbidden(t *testing.T) {
	schools := school.NewSchoolService(&fakeSchoolRepo{assigned: map[uint][]models.School{}})
	e := echo.New()
	stashPrincipal(e, 7, false)
	e.GET("/x", func(c echo.Context) error { return c.String(http.StatusOK, "ok") }, routes.EchoRequirePlatformAdmin(schools))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("staff must be forbidden from platform routes; status=%d", rec.Code)
	}
}

// EchoRequireAdmin は職員を弾き、管理者を通す。
func TestRequireAdmin_StaffForbidden_AdminOK(t *testing.T) {
	for _, tc := range []struct {
		name       string
		isPlatform bool
		want       int
	}{
		{"staff", false, http.StatusForbidden},
		{"admin", true, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			stashPrincipal(e, 1, tc.isPlatform)
			e.GET("/x", func(c echo.Context) error { return c.String(http.StatusOK, "ok") }, routes.EchoRequireAdmin())
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
			if rec.Code != tc.want {
				t.Fatalf("%s: status=%d want=%d", tc.name, rec.Code, tc.want)
			}
		})
	}
}
