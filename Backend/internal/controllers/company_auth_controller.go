package controllers

import (
	companyauth "Backend/internal/services/companyauth"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

type CompanyAuthController struct {
	svc *companyauth.CompanyUserService
}

func NewCompanyAuthController(svc *companyauth.CompanyUserService) *CompanyAuthController {
	return &CompanyAuthController{svc: svc}
}

func (c *CompanyAuthController) Login(ctx echo.Context) error {
	var req companyauth.LoginRequest
	if err := ctx.Bind(&req); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}
	resp, err := c.svc.Login(req)
	if err != nil {
		if errors.Is(err, companyauth.ErrInvalidCredentials) {
			return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "invalid email or password")
		}
		return mapCompanyAuthError(err)
	}
	return ctx.JSON(http.StatusOK, resp)
}

func (c *CompanyAuthController) AcceptInvite(ctx echo.Context) error {
	var req companyauth.AcceptInviteRequest
	if err := ctx.Bind(&req); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}
	resp, err := c.svc.AcceptInvite(req)
	if err != nil {
		return mapCompanyAuthError(err)
	}
	return ctx.JSON(http.StatusOK, resp)
}

func (c *CompanyAuthController) Me(ctx echo.Context) error {
	companyUserID, ok := echoCompanyUserID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}
	resp, err := c.svc.GetMe(companyUserID)
	if err != nil {
		return mapCompanyAuthError(err)
	}
	return ctx.JSON(http.StatusOK, resp)
}

func mapCompanyAuthError(err error) error {
	switch {
	case errors.Is(err, companyauth.ErrInvalidCredentials):
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "invalid email or password")
	case errors.Is(err, companyauth.ErrInviteNotFound):
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "invalid invite token")
	case errors.Is(err, companyauth.ErrInviteExpired):
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "invite token expired")
	case errors.Is(err, companyauth.ErrEmailExists):
		return newAPIError(http.StatusConflict, ErrCodeConflict, "email already exists")
	case errors.Is(err, companyauth.ErrCompanyNotFound):
		return newAPIError(http.StatusNotFound, ErrCodeNotFound, "company not found")
	case errors.Is(err, companyauth.ErrCompanyNotVerified):
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "company is not verified")
	default:
		msg := err.Error()
		if msg == "forbidden" {
			return newAPIError(http.StatusForbidden, ErrCodeForbidden, "Forbidden")
		}
		if msg == "email and password are required" || msg == "token and password are required" ||
			msg == "password must be at least 8 characters" || msg == "email and name are required" ||
			msg == "email is invalid" || msg == "invalid role" || msg == "invite already accepted" {
			return newAPIError(http.StatusBadRequest, ErrCodeValidationError, msg)
		}
		return echoInternalError(err)
	}
}
