package admin_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"
)

func TestAdminCompanyController_Names_ServiceError(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindAllActiveNames", "").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies/names", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminCompanyController(repo, nil).Names, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Names_Success(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	list := []models.CompanyName{{ID: 1, Name: "Test Corp"}}
	repo.On("FindAllActiveNames", "Test").Return(list, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies/names?q=Test", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminCompanyController(repo, nil).Names, testsupport.NewCtx(req, rec), http.StatusOK)
	repo.AssertExpectations(t)
}
