package routes_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/routes"
	"Backend/internal/services"

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

func newSchoolScopeTestEcho(schools *services.SchoolService, adminUserID uint) (*echo.Echo, *httptest.ResponseRecorder) {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := context.WithValue(c.Request().Context(), middleware.AdminUserIDContextKey, adminUserID)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})
	e.GET("/x", func(c echo.Context) error { return c.String(http.StatusOK, "ok") }, routes.EchoAdminSchoolScope(schools))
	return e, httptest.NewRecorder()
}

func TestEchoAdminSchoolScope_UnrestrictedNoFilter(t *testing.T) {
	schools := services.NewSchoolService(&fakeSchoolRepo{assigned: map[uint][]models.School{}})
	e, rec := newSchoolScopeTestEcho(schools, 1)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestEchoAdminSchoolScope_RestrictedMissingSchoolID(t *testing.T) {
	schools := services.NewSchoolService(&fakeSchoolRepo{assigned: map[uint][]models.School{
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
	schools := services.NewSchoolService(&fakeSchoolRepo{assigned: map[uint][]models.School{
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
	schools := services.NewSchoolService(&fakeSchoolRepo{assigned: map[uint][]models.School{
		2: {{ID: 5}},
	}})
	e, rec := newSchoolScopeTestEcho(schools, 2)

	req := httptest.NewRequest(http.MethodGet, "/x?school_id=5", nil)
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
