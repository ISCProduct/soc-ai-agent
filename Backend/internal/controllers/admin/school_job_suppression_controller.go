package admin

// キャリア担当による個別求人停止（#1508）。
//
// 企業の掲載承認は企業単位だが、問題のある求人だけを学校向けに個別停止できる。
// 担当校スコープを ensureSchoolAccess で確認する（#1157）。

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

type suppressionService interface {
	List(schoolID uint) ([]models.SchoolJobSuppression, error)
	Suppress(schoolID, jobPositionID, actorID uint, reason string) (*models.SchoolJobSuppression, error)
	Unsuppress(schoolID, jobPositionID uint) error
}

// AdminSchoolJobSuppressionController は個別求人停止を扱う。
type AdminSchoolJobSuppressionController struct {
	suppress suppressionService
	schools  *school.SchoolService
}

func NewAdminSchoolJobSuppressionController(suppress suppressionService, schools *school.SchoolService) *AdminSchoolJobSuppressionController {
	return &AdminSchoolJobSuppressionController{suppress: suppress, schools: schools}
}

type suppressJobRequest struct {
	JobPositionID uint   `json:"job_position_id"`
	Reason        string `json:"reason"`
}

// List GET /api/admin/schools/:id/job-suppressions
func (c *AdminSchoolJobSuppressionController) List(ctx echo.Context) error {
	schoolID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return err
	}
	if forbidden := c.ensureSchoolAccess(ctx, schoolID); forbidden != nil {
		return forbidden
	}
	list, err := c.suppress.List(schoolID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"suppressions": list})
}

// Create POST /api/admin/schools/:id/job-suppressions
func (c *AdminSchoolJobSuppressionController) Create(ctx echo.Context) error {
	schoolID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return err
	}
	if forbidden := c.ensureSchoolAccess(ctx, schoolID); forbidden != nil {
		return forbidden
	}
	var req suppressJobRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.JobPositionID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "job_position_id is required")
	}
	actorID, ok := middleware.AdminUserIDFromContext(ctx.Request().Context())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	sup, err := c.suppress.Suppress(schoolID, req.JobPositionID, actorID, req.Reason)
	if err != nil {
		if errors.Is(err, schoolapproval.ErrAlreadySuppressed) {
			return echo.NewHTTPError(http.StatusConflict, err.Error())
		}
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusCreated, sup)
}

// Delete DELETE /api/admin/schools/:id/job-suppressions/:jobId
func (c *AdminSchoolJobSuppressionController) Delete(ctx echo.Context) error {
	schoolID, err := httpapi.UintParam(ctx, "id")
	if err != nil {
		return err
	}
	if forbidden := c.ensureSchoolAccess(ctx, schoolID); forbidden != nil {
		return forbidden
	}
	jobID, err := httpapi.UintParam(ctx, "jobId")
	if err != nil {
		return err
	}
	if err := c.suppress.Unsuppress(schoolID, jobID); err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.NoContent(http.StatusNoContent)
}

// ensureSchoolAccess は担当校スコープを確認する（#1157）。
func (c *AdminSchoolJobSuppressionController) ensureSchoolAccess(ctx echo.Context, schoolID uint) error {
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
