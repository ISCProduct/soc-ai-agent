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
	// 担当校を持つ管理者が任意の学校へ自分を追加できると、他校の生徒データを
	// 「正規の担当」として閲覧できてしまう(#1157)
	if err := c.ensureSchoolAccess(ctx, id); err != nil {
		return err
	}
	adminUserID, ok := middleware.AdminUserIDFromContext(ctx.Request().Context())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	callerRestricted, _, err := c.schools.ResolveAdminAccess(adminUserID)
	if err != nil {
		return echoInternalError(err)
	}
	if callerRestricted {
		// 担当校を持たないユーザー（無制限管理者・新任の先生・一般ユーザー）の追加は
		// 無制限管理者に限る。無制限管理者を自校の担当に追加すると担当校1件の制限adminへ
		// 降格し、最後の担当校は制限adminからは外せないため（下記 RemoveMember のガード）、
		// 無制限管理者を恒久的に自校へ閉じ込められてしまう(#1157)。
		targetRestricted, _, err := c.schools.ResolveAdminAccess(req.UserID)
		if err != nil {
			return echoInternalError(err)
		}
		if !targetRestricted {
			return echo.NewHTTPError(http.StatusForbidden,
				"担当校をまだ持たないユーザーの追加は無制限管理者のみ可能です")
		}
	} else if req.UserID == adminUserID {
		// 無制限管理者が自分を担当に追加すると制限adminへ降格し、
		// 最後の担当校は自分では外せない（他に無制限管理者が居なければ手SQL以外で戻せない）
		return echo.NewHTTPError(http.StatusForbidden,
			"自分自身を担当校へ追加できません（無制限管理者の権限を失い、自力で戻せなくなります）")
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
	if err := c.ensureSchoolAccess(ctx, id); err != nil {
		return err
	}
	adminUserID, ok := middleware.AdminUserIDFromContext(ctx.Request().Context())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	// 担当校が0件になる削除を、担当校を持つ管理者(制限admin)には禁止する(#1157)。
	//
	// ResolveAdminAccess は担当校0件を「無制限のプラットフォーム管理者」として扱うため、
	// 最後の担当校を外すと権限剥奪のつもりが全校アクセスへの昇格になる。
	// 一方で無制限管理者まで止めると、退職した先生の担当を外す正当な運用が
	// API から行えなくなる（ResolveAdminAccess は is_admin を見ないため、
	// is_admin を落としても担当校ありのままで、回避手順が存在しない）。
	// 昇格が成立するのは制限adminが実行する場合だけなので、そこだけを拒否する。
	callerRestricted, _, err := c.schools.ResolveAdminAccess(adminUserID)
	if err != nil {
		return echoInternalError(err)
	}
	if callerRestricted {
		targetRestricted, targetSchools, err := c.schools.ResolveAdminAccess(userID)
		if err != nil {
			return echoInternalError(err)
		}
		if targetRestricted && len(targetSchools) == 1 && targetSchools[0] == id {
			return echo.NewHTTPError(http.StatusForbidden,
				"最後の担当校は解除できません（担当校0件は無制限管理者として扱われます）。無制限管理者に依頼してください")
		}
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
