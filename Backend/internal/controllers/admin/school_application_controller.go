package admin

// キャリア担当による掲載申請の審査（#1507）。
//
// 企業が出した掲載申請（#1506）を、学校のキャリア担当職員が承認・却下する。
// 担当校スコープを ensureSchoolAccess で確認し、他校の申請には触れさせない（#1157）。

import (
	"errors"
	"net/http"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/services/school"
	"Backend/internal/services/schoolapproval"

	"github.com/labstack/echo/v4"
)

type reviewService interface {
	List(schoolID uint, status string) ([]models.SchoolCompanyApplication, error)
	Approve(appID, schoolID, reviewerID uint) (*models.SchoolCompanyApplication, error)
	Reject(appID, schoolID, reviewerID uint) (*models.SchoolCompanyApplication, error)
}

// AdminSchoolApplicationController は掲載申請の審査キューを扱う。
type AdminSchoolApplicationController struct {
	reviews reviewService
	schools *school.SchoolService
}

func NewAdminSchoolApplicationController(reviews reviewService, schools *school.SchoolService) *AdminSchoolApplicationController {
	return &AdminSchoolApplicationController{reviews: reviews, schools: schools}
}

// List GET /api/admin/schools/:id/company-applications?status=pending
func (c *AdminSchoolApplicationController) List(ctx echo.Context) error {
	schoolID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return err
	}
	if forbidden := c.ensureSchoolAccess(ctx, schoolID); forbidden != nil {
		return forbidden
	}
	apps, err := c.reviews.List(schoolID, ctx.QueryParam("status"))
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"applications": apps})
}

// Approve POST /api/admin/schools/:id/company-applications/:appId/approve
func (c *AdminSchoolApplicationController) Approve(ctx echo.Context) error {
	return c.review(ctx, func(appID, schoolID, reviewerID uint) (*models.SchoolCompanyApplication, error) {
		return c.reviews.Approve(appID, schoolID, reviewerID)
	})
}

// Reject POST /api/admin/schools/:id/company-applications/:appId/reject
func (c *AdminSchoolApplicationController) Reject(ctx echo.Context) error {
	return c.review(ctx, func(appID, schoolID, reviewerID uint) (*models.SchoolCompanyApplication, error) {
		return c.reviews.Reject(appID, schoolID, reviewerID)
	})
}

func (c *AdminSchoolApplicationController) review(
	ctx echo.Context,
	do func(appID, schoolID, reviewerID uint) (*models.SchoolCompanyApplication, error),
) error {
	schoolID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return err
	}
	if forbidden := c.ensureSchoolAccess(ctx, schoolID); forbidden != nil {
		return forbidden
	}
	appID, err := httpapi.UintParam(ctx, "appId")
	if err != nil {
		return err
	}
	reviewerID, ok := middleware.AdminUserIDFromContext(ctx.Request().Context())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	app, err := do(appID, schoolID, reviewerID)
	if err != nil {
		return mapReviewError(err)
	}
	return ctx.JSON(http.StatusOK, app)
}

// ensureSchoolAccess は担当校スコープを確認する（AdminSchoolController と同方針）。
func (c *AdminSchoolApplicationController) ensureSchoolAccess(ctx echo.Context, schoolID uint) error {
	adminUserID, ok := middleware.AdminUserIDFromContext(ctx.Request().Context())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	allowed, err := c.schools.CanAdminAccessSchool(adminUserID, &schoolID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	if !allowed {
		return echo.NewHTTPError(http.StatusForbidden, "school access denied")
	}
	return nil
}

func mapReviewError(err error) error {
	switch {
	case errors.Is(err, schoolapproval.ErrApplicationNotFound):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, schoolapproval.ErrNotPending):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	default:
		return httpapi.InternalError(err)
	}
}
