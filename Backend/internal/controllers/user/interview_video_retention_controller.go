package user

import (
	"errors"
	"net/http"

	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/services/school"
	"Backend/internal/services/videoretention"

	"github.com/labstack/echo/v4"
)

type retentionUserLookup interface {
	GetUserByID(id uint) (*models.User, error)
}

type retentionSchoolLookup interface {
	Get(id uint) (*models.School, error)
}

// InterviewVideoRetentionController は、本人の学校が選んだ録画の保持期間を返す。
type InterviewVideoRetentionController struct {
	users   retentionUserLookup
	schools retentionSchoolLookup
}

func NewInterviewVideoRetentionController(users retentionUserLookup, schools retentionSchoolLookup) *InterviewVideoRetentionController {
	return &InterviewVideoRetentionController{users: users, schools: schools}
}

func (c *InterviewVideoRetentionController) Get(ctx echo.Context) error {
	userID, ok := middleware.UserIDFromContext(ctx.Request().Context())
	if !ok || userID == 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	student, err := c.users.GetUserByID(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load user")
	}
	var foundSchool *models.School
	if student != nil && student.SchoolID != nil && c.schools != nil {
		found, err := c.schools.Get(*student.SchoolID)
		if err != nil && !errors.Is(err, school.ErrSchoolNotFound) {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to load school")
		}
		foundSchool = found
	}
	days, unlimited := videoretention.Resolve(foundSchool)
	return ctx.JSON(http.StatusOK, map[string]any{
		"days":      days,
		"unlimited": unlimited,
	})
}
