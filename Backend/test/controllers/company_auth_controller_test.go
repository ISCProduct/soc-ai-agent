package controllers_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	companyauth "Backend/internal/services/companyauth"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestCompanyAuthController_Login_InvalidBody(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/company-auth/login", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := controllers.NewCompanyAuthController(&companyauth.CompanyUserService{}).Login(c)
	assert.Error(t, err)
}

func TestCompanyAuthController_Me_Unauthorized(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/company-auth/me", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := controllers.NewCompanyAuthController(&companyauth.CompanyUserService{}).Me(c)
	assert.Error(t, err)
}
