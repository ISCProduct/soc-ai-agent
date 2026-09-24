package company

// 企業ポータルの掲載申請導線（#1506）。
//
// 企業が「この学校に求人を掲載したい」と申請し、学校のキャリア担当が承認する。
// company_id は JWT 由来。ボディに company_id があっても無視する。
// 他社の申請IDを取り消そうとした場合は 404 ではなく 403 を返す（#1156）。
// 申請・取消は破壊的操作なので owner のみ（#1319 の共通方針）。

import (
	"errors"
	"net/http"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/services/companyportal"
	"Backend/internal/services/shared"

	"github.com/labstack/echo/v4"
)

type portalSchoolApplicationService interface {
	List(companyID uint) ([]models.SchoolCompanyApplication, error)
	Apply(companyID, schoolID, appliedBy uint) (*models.SchoolCompanyApplication, error)
	Cancel(id, companyID uint) error
}

type CompanyPortalSchoolApplicationController struct {
	apps portalSchoolApplicationService
}

func NewCompanyPortalSchoolApplicationController(apps portalSchoolApplicationService) *CompanyPortalSchoolApplicationController {
	return &CompanyPortalSchoolApplicationController{apps: apps}
}

type schoolApplicationResponse struct {
	ID        uint   `json:"id"`
	SchoolID  uint   `json:"school_id"`
	CompanyID uint   `json:"company_id"`
	Status    string `json:"status"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func toSchoolApplicationResponse(a *models.SchoolCompanyApplication) schoolApplicationResponse {
	return schoolApplicationResponse{
		ID:        a.ID,
		SchoolID:  a.SchoolID,
		CompanyID: a.CompanyID,
		Status:    a.Status,
		Note:      a.Note,
		CreatedAt: a.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: a.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

type schoolApplicationBody struct {
	SchoolID uint `json:"school_id"`
}

// List GET /api/company-portal/school-applications
func (c *CompanyPortalSchoolApplicationController) List(ctx echo.Context) error {
	companyID, apiErr := c.requireOwner(ctx)
	if apiErr != nil {
		return apiErr
	}
	apps, err := c.apps.List(companyID)
	if err != nil {
		return mapSchoolApplicationError(err)
	}
	res := make([]schoolApplicationResponse, 0, len(apps))
	for i := range apps {
		res = append(res, toSchoolApplicationResponse(&apps[i]))
	}
	return ctx.JSON(http.StatusOK, res)
}

// Create POST /api/company-portal/school-applications （owner のみ）
func (c *CompanyPortalSchoolApplicationController) Create(ctx echo.Context) error {
	companyID, apiErr := c.requireOwner(ctx)
	if apiErr != nil {
		return apiErr
	}
	userID, ok := httpapi.CompanyUserID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	var body schoolApplicationBody
	if err := ctx.Bind(&body); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	app, err := c.apps.Apply(companyID, body.SchoolID, userID)
	if err != nil {
		return mapSchoolApplicationError(err)
	}
	return ctx.JSON(http.StatusCreated, toSchoolApplicationResponse(app))
}

// Delete DELETE /api/company-portal/school-applications/:id （owner のみ）
func (c *CompanyPortalSchoolApplicationController) Delete(ctx echo.Context) error {
	companyID, apiErr := c.requireOwner(ctx)
	if apiErr != nil {
		return apiErr
	}
	id, apiErr := httpapi.UintParam(ctx, "id")
	if apiErr != nil {
		return apiErr
	}
	if err := c.apps.Cancel(id, companyID); err != nil {
		return mapSchoolApplicationError(err)
	}
	return ctx.NoContent(http.StatusNoContent)
}

// requireOwner は owner 権限を確認し、JWT 由来の company_id を返す。
func (c *CompanyPortalSchoolApplicationController) requireOwner(ctx echo.Context) (uint, error) {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return 0, httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	if !middleware.CompanyUserIsOwner(ctx.Request().Context()) {
		return 0, httpapi.NewAPIError(http.StatusForbidden, httpapi.ErrCodeForbidden, "掲載申請の管理は管理者のみ行えます")
	}
	return companyID, nil
}

func mapSchoolApplicationError(err error) error {
	var ve *shared.ValidationError
	switch {
	case errors.Is(err, shared.ErrForbidden):
		return httpapi.NewAPIError(http.StatusForbidden, httpapi.ErrCodeForbidden, "この申請を操作する権限がありません")
	case errors.Is(err, companyportal.ErrDuplicateApplication):
		return httpapi.NewAPIError(http.StatusConflict, httpapi.ErrCodeInvalidStatus, err.Error())
	case errors.As(err, &ve):
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, ve.Message)
	default:
		return httpapi.NewAPIError(http.StatusInternalServerError, httpapi.ErrCodeInternalError, "掲載申請の処理に失敗しました")
	}
}
