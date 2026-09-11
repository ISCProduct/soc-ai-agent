package controllers

import (
	"Backend/internal/middleware"
	"Backend/internal/services"
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// AdminSchoolController はプラットフォーム管理者向けの個別校CRUD、担当管理者割当、
// 企業掲載承認リストを扱う。
type AdminSchoolController struct {
	schools *services.SchoolService
}

func NewAdminSchoolController(schools *services.SchoolService) *AdminSchoolController {
	return &AdminSchoolController{schools: schools}
}

type createSchoolRequest struct {
	OrganizationID uint   `json:"organization_id"`
	Name           string `json:"name"`
}

type addSchoolMemberRequest struct {
	UserID uint `json:"user_id"`
}

type addCompanyApprovalRequest struct {
	CompanyID uint `json:"company_id"`
}

// List GET /api/admin/schools
func (c *AdminSchoolController) List(ctx echo.Context) error {
	limit := 25
	offset := 0
	if l, err := strconv.Atoi(ctx.QueryParam("limit")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	if o, err := strconv.Atoi(ctx.QueryParam("offset")); err == nil && o >= 0 {
		offset = o
	}
	schools, total, err := c.schools.List(limit, offset)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"schools": schools,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// Create POST /api/admin/schools
func (c *AdminSchoolController) Create(ctx echo.Context) error {
	var req createSchoolRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	school, err := c.schools.Create(services.CreateSchoolInput{OrganizationID: req.OrganizationID, Name: req.Name})
	if err != nil {
		return mapSchoolError(err)
	}
	return ctx.JSON(http.StatusCreated, school)
}

// Get GET /api/admin/schools/:id
func (c *AdminSchoolController) Get(ctx echo.Context) error {
	id, err := echoUintParam(ctx, "id")
	if err != nil {
		return err
	}
	school, err := c.schools.Get(id)
	if err != nil {
		return mapSchoolError(err)
	}
	return ctx.JSON(http.StatusOK, school)
}

// AddMember POST /api/admin/schools/:id/members
func (c *AdminSchoolController) AddMember(ctx echo.Context) error {
	id, err := echoUintParam(ctx, "id")
	if err != nil {
		return err
	}
	var req addSchoolMemberRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.UserID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "user_id is required")
	}
	if err := c.schools.AddMember(req.UserID, id); err != nil {
		return mapSchoolError(err)
	}
	return ctx.JSON(http.StatusCreated, map[string]string{"message": "member added"})
}

// RemoveMember DELETE /api/admin/schools/:id/members/:user_id
func (c *AdminSchoolController) RemoveMember(ctx echo.Context) error {
	id, err := echoUintParam(ctx, "id")
	if err != nil {
		return err
	}
	userID, err := echoUintParam(ctx, "user_id")
	if err != nil {
		return err
	}
	if err := c.schools.RemoveMember(userID, id); err != nil {
		return mapSchoolError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]string{"message": "member removed"})
}

// ListCompanyApprovals GET /api/admin/schools/:id/company-approvals
func (c *AdminSchoolController) ListCompanyApprovals(ctx echo.Context) error {
	id, err := echoUintParam(ctx, "id")
	if err != nil {
		return err
	}
	ids, err := c.schools.ListApprovedCompanyIDs(id)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"company_ids": ids})
}

// AddCompanyApproval POST /api/admin/schools/:id/company-approvals
func (c *AdminSchoolController) AddCompanyApproval(ctx echo.Context) error {
	id, err := echoUintParam(ctx, "id")
	if err != nil {
		return err
	}
	if forbidden := c.ensureSchoolAccess(ctx, id); forbidden != nil {
		return forbidden
	}
	var req addCompanyApprovalRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.CompanyID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "company_id is required")
	}
	if err := c.schools.AddCompanyApproval(id, req.CompanyID); err != nil {
		return mapSchoolError(err)
	}
	return ctx.JSON(http.StatusCreated, map[string]string{"message": "company approved"})
}

// RemoveCompanyApproval DELETE /api/admin/schools/:id/company-approvals/:company_id
func (c *AdminSchoolController) RemoveCompanyApproval(ctx echo.Context) error {
	id, err := echoUintParam(ctx, "id")
	if err != nil {
		return err
	}
	if forbidden := c.ensureSchoolAccess(ctx, id); forbidden != nil {
		return forbidden
	}
	companyID, err := echoUintParam(ctx, "company_id")
	if err != nil {
		return err
	}
	if err := c.schools.RemoveCompanyApproval(id, companyID); err != nil {
		return mapSchoolError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]string{"message": "approval removed"})
}

// ensureSchoolAccess は承認リスト操作の権限(無制限、またはその学校の担当)を検証する。
func (c *AdminSchoolController) ensureSchoolAccess(ctx echo.Context, schoolID uint) error {
	adminUserID, ok := middleware.AdminUserIDFromContext(ctx.Request().Context())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	restricted, allowedSchoolIDs, err := c.schools.ResolveAdminAccess(adminUserID)
	if err != nil {
		return echoInternalError(err)
	}
	if !restricted {
		return nil
	}
	for _, id := range allowedSchoolIDs {
		if id == schoolID {
			return nil
		}
	}
	return echo.NewHTTPError(http.StatusForbidden, "school access denied")
}

// MySchoolAccess GET /api/admin/me/school-access
// フロントの学校フィルタUI用に、自分が選べる学校の一覧と無制限かどうかを返す。
func (c *AdminSchoolController) MySchoolAccess(ctx echo.Context) error {
	adminUserID, ok := middleware.AdminUserIDFromContext(ctx.Request().Context())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	restricted, schools, err := c.schools.ListAccessibleSchools(adminUserID)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"restricted": restricted,
		"schools":    schools,
	})
}

func mapSchoolError(err error) error {
	switch {
	case errors.Is(err, services.ErrSchoolNotFound):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, services.ErrSchoolAlreadyAssigned), errors.Is(err, services.ErrCompanyAlreadyApproved):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, services.ErrSchoolNameRequired), errors.Is(err, services.ErrSchoolOrgRequired):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		return echoInternalError(err)
	}
}
