package controllers

import (
	"Backend/internal/services"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

// AdminOrganizationController はプラットフォーム管理者向け組織 CRUD。
type AdminOrganizationController struct {
	orgs *services.OrganizationService
}

func NewAdminOrganizationController(orgs *services.OrganizationService) *AdminOrganizationController {
	return &AdminOrganizationController{orgs: orgs}
}

type createOrganizationRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type updateOrganizationRequest struct {
	Name   *string `json:"name"`
	Status *string `json:"status"`
}

type addMemberRequest struct {
	UserID uint   `json:"user_id"`
	Role   string `json:"role"`
}

type updateMemberRequest struct {
	Role string `json:"role"`
}

// List GET /api/admin/organizations
func (c *AdminOrganizationController) List(ctx echo.Context) error {
	limit := 25
	offset := 0
	if l, err := strconv.Atoi(ctx.QueryParam("limit")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	if o, err := strconv.Atoi(ctx.QueryParam("offset")); err == nil && o >= 0 {
		offset = o
	}
	orgs, total, err := c.orgs.List(limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list organizations")
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"organizations": orgs,
		"total":         total,
		"limit":         limit,
		"offset":        offset,
	})
}

// Create POST /api/admin/organizations
func (c *AdminOrganizationController) Create(ctx echo.Context) error {
	var req createOrganizationRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	org, err := c.orgs.Create(services.CreateOrganizationInput{Name: req.Name, Slug: req.Slug})
	if err != nil {
		return mapOrgError(err)
	}
	return ctx.JSON(http.StatusCreated, org)
}

// Get GET /api/admin/organizations/:id
func (c *AdminOrganizationController) Get(ctx echo.Context) error {
	id, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid organization id")
	}
	org, err := c.orgs.Get(id)
	if err != nil {
		return mapOrgError(err)
	}
	return ctx.JSON(http.StatusOK, org)
}

// Update PUT /api/admin/organizations/:id
func (c *AdminOrganizationController) Update(ctx echo.Context) error {
	id, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid organization id")
	}
	var req updateOrganizationRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	org, err := c.orgs.Update(id, services.UpdateOrganizationInput{Name: req.Name, Status: req.Status})
	if err != nil {
		return mapOrgError(err)
	}
	return ctx.JSON(http.StatusOK, org)
}

// ListMembers GET /api/admin/organizations/:id/members
func (c *AdminOrganizationController) ListMembers(ctx echo.Context) error {
	id, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid organization id")
	}
	members, err := c.orgs.ListMembers(id)
	if err != nil {
		return mapOrgError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"members": members})
}

// AddMember POST /api/admin/organizations/:id/members
func (c *AdminOrganizationController) AddMember(ctx echo.Context) error {
	id, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid organization id")
	}
	var req addMemberRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.UserID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "user_id is required")
	}
	member, err := c.orgs.AddMember(services.AddMemberInput{
		OrganizationID: id,
		UserID:         req.UserID,
		Role:           req.Role,
	})
	if err != nil {
		return mapOrgError(err)
	}
	return ctx.JSON(http.StatusCreated, member)
}

// UpdateMember PUT /api/admin/organizations/:id/members/:user_id
func (c *AdminOrganizationController) UpdateMember(ctx echo.Context) error {
	orgID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid organization id")
	}
	userID, err := echoUintParam(ctx, "user_id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}
	var req updateMemberRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	member, err := c.orgs.UpdateMemberRole(orgID, userID, req.Role)
	if err != nil {
		return mapOrgError(err)
	}
	return ctx.JSON(http.StatusOK, member)
}

// RemoveMember DELETE /api/admin/organizations/:id/members/:user_id
func (c *AdminOrganizationController) RemoveMember(ctx echo.Context) error {
	orgID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid organization id")
	}
	userID, err := echoUintParam(ctx, "user_id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}
	if err := c.orgs.RemoveMember(orgID, userID); err != nil {
		return mapOrgError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]string{"message": "member removed"})
}

func mapOrgError(err error) *echo.HTTPError {
	switch {
	case errors.Is(err, services.ErrOrganizationNotFound), errors.Is(err, services.ErrMembershipNotFound):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, services.ErrOrgSlugTaken), errors.Is(err, services.ErrUserAlreadyInOrg):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, services.ErrInvalidOrgSlug), errors.Is(err, services.ErrInvalidOrgRole):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case errors.Is(err, services.ErrCrossOrganization):
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	default:
		if err != nil && (strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "must be")) {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "organization operation failed")
	}
}
