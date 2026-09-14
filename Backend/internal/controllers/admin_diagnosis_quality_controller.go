package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/repositories"
	"Backend/internal/services"
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// AdminDiagnosisQualityController は診断妥当性レポートの参照API。
type AdminDiagnosisQualityController struct {
	repo     *repositories.DiagnosisQualityRepository
	userRepo repository.UserRepository
	schools  *services.SchoolService
}

func NewAdminDiagnosisQualityController(
	repo *repositories.DiagnosisQualityRepository,
	userRepo repository.UserRepository,
	schools *services.SchoolService,
) *AdminDiagnosisQualityController {
	return &AdminDiagnosisQualityController{repo: repo, userRepo: userRepo, schools: schools}
}

// GetBySession GET /api/admin/diagnosis-quality?user_id=&session_id=
func (c *AdminDiagnosisQualityController) GetBySession(ctx echo.Context) error {
	sessionID := ctx.QueryParam("session_id")
	if sessionID == "" {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "session_id is required"})
	}
	userID64, err := strconv.ParseUint(ctx.QueryParam("user_id"), 10, 64)
	if err != nil || userID64 == 0 {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "user_id is required"})
	}
	userID := uint(userID64)
	if err := c.ensureTargetUserSchoolAccess(ctx, userID); err != nil {
		return err
	}
	report, err := c.repo.FindByUserAndSession(userID, sessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
		}
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, report)
}

// List GET /api/admin/diagnosis-quality?limit=
func (c *AdminDiagnosisQualityController) List(ctx echo.Context) error {
	if ctx.QueryParam("session_id") != "" {
		return c.GetBySession(ctx)
	}
	schoolFilter, err := echoAdminSchoolFilter(ctx)
	if err != nil {
		return err
	}
	limit, _ := strconv.Atoi(ctx.QueryParam("limit"))
	rows, err := c.repo.ListRecent(limit, schoolFilter)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": rows})
}

func (c *AdminDiagnosisQualityController) ensureTargetUserSchoolAccess(ctx echo.Context, userID uint) error {
	if c.userRepo == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user repository is not configured")
	}
	owner, err := c.userRepo.GetUserByID(userID)
	if err != nil || owner == nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	return ensureAdminSchoolAccess(ctx, c.schools, owner.SchoolID)
}
