package controllers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/models"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/mock"
)

func TestAdminCompanyController_Names_ServiceError(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindAllActiveNames", "").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies/names", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newAdminCompanyController(repo, nil).Names, newCtx(req, rec), http.StatusInternalServerError)
	repo.AssertExpectations(t)
}

func TestAdminCompanyController_Names_Success(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	list := []models.CompanyName{{ID: 1, Name: "Test Corp"}}
	repo.On("FindAllActiveNames", "Test").Return(list, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/companies/names?q=Test", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newAdminCompanyController(repo, nil).Names, newCtx(req, rec), http.StatusOK)
	repo.AssertExpectations(t)
}
