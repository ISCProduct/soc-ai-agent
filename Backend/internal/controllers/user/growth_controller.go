package user

import (
	"errors"
	"net/http"
	"strconv"

	"Backend/domain/repository"
	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/services/growth"
	"Backend/internal/services/school"

	"github.com/labstack/echo/v4"
)

// SnapshotLister は本人の成長の記録を新しい順に読む。
type SnapshotLister interface {
	ListRecent(userID uint, limit int) ([]models.UserGrowthSnapshot, error)
}

type userLookup interface {
	GetUserByID(id uint) (*models.User, error)
}

// GrowthController はスコアと診断フェーズの履歴を返す（SOCAIAGENT-601）。
// 学生はトークンの本人だけ。職員は担当校の学生だけ。クエリの user_id は見ない。
type GrowthController struct {
	snapshots SnapshotLister
	users     userLookup
	schools   *school.SchoolService
}

func NewGrowthController(snapshots SnapshotLister, users userLookup, schools *school.SchoolService) *GrowthController {
	return &GrowthController{snapshots: snapshots, users: users, schools: schools}
}

func (c *GrowthController) ForStudent(ctx echo.Context) error {
	userID, ok := middleware.UserIDFromContext(ctx.Request().Context())
	if !ok || userID == 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	return c.write(ctx, userID)
}

func (c *GrowthController) ForStaff(ctx echo.Context) error {
	if _, err := httpapi.AdminSchoolFilter(ctx); err != nil {
		return err
	}
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	student, err := c.users.GetUserByID(uint(id))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		httpapi.LogError(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load user")
	}
	if student == nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	if err := httpapi.EnsureAdminSchoolAccessForUser(ctx, c.schools, student.OrganizationID, student.SchoolID); err != nil {
		return err
	}
	return c.write(ctx, student.ID)
}

func (c *GrowthController) write(ctx echo.Context, userID uint) error {
	rows, err := c.snapshots.ListRecent(userID, 30)
	if err != nil {
		httpapi.LogError(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load growth history")
	}
	return ctx.JSON(http.StatusOK, growth.BuildHistory(rows))
}
