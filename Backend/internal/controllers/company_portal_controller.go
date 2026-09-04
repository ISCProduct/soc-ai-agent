package controllers

import (
	companyauth "Backend/internal/services/companyauth"
	"net/http"

	"github.com/labstack/echo/v4"
)

type AdminCompanyUserController struct {
	svc *companyauth.CompanyUserService
}

func NewAdminCompanyUserController(svc *companyauth.CompanyUserService) *AdminCompanyUserController {
	return &AdminCompanyUserController{svc: svc}
}

func (c *AdminCompanyUserController) Invite(ctx echo.Context) error {
	companyID, err := echoUintParam(ctx, "id")
	if err != nil {
		return err
	}
	var req companyauth.InviteRequest
	if err := ctx.Bind(&req); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}
	user, err := c.svc.Invite(companyID, req)
	if err != nil {
		return mapCompanyAuthError(err)
	}
	return ctx.JSON(http.StatusCreated, map[string]any{
		"id":         user.ID,
		"company_id": user.CompanyID,
		"email":      user.Email,
		"name":       user.Name,
		"role":       user.Role,
	})
}

func (c *AdminCompanyUserController) List(ctx echo.Context) error {
	companyID, err := echoUintParam(ctx, "id")
	if err != nil {
		return err
	}
	users, err := c.svc.ListByCompany(companyID)
	if err != nil {
		return echoInternalError(err)
	}
	items := make([]map[string]any, 0, len(users))
	for _, u := range users {
		items = append(items, map[string]any{
			"id":             u.ID,
			"company_id":     u.CompanyID,
			"email":          u.Email,
			"name":           u.Name,
			"role":           u.Role,
			"password_set":   u.PasswordSet(),
			"invite_pending": !u.PasswordSet(),
		})
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": items})
}

type CompanyPortalController struct {
	svc *companyauth.CompanyUserService
}

func NewCompanyPortalController(svc *companyauth.CompanyUserService) *CompanyPortalController {
	return &CompanyPortalController{svc: svc}
}

// GetCompany は自社 company_id のみ閲覧可能（他社は 403）。
func (c *CompanyPortalController) GetCompany(ctx echo.Context) error {
	companyUserID, ok := echoCompanyUserID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}
	companyID, err := echoUintParam(ctx, "id")
	if err != nil {
		return err
	}
	if err := c.svc.EnsureCompanyAccess(companyUserID, companyID); err != nil {
		if err.Error() == "forbidden" {
			return newAPIError(http.StatusForbidden, ErrCodeForbidden, "Forbidden")
		}
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"company_id": companyID,
		"message":    "ok",
	})
}
