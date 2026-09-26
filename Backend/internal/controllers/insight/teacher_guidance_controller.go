package insight

import (
	"errors"
	"net/http"
	"strconv"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/services/school"
	"Backend/internal/services/teacher"

	"github.com/labstack/echo/v4"
)

type TeacherGuidanceController struct {
	svc     *teacher.GuidanceService
	schools *school.SchoolService
}

func NewTeacherGuidanceController(svc *teacher.GuidanceService, schools *school.SchoolService) *TeacherGuidanceController {
	return &TeacherGuidanceController{svc: svc, schools: schools}
}

type createGuidanceRequest struct {
	Kind                string   `json:"kind"`
	Message             string   `json:"message"`
	SuggestedIndustries []string `json:"suggested_industries"`
}

// Create POST /api/admin/teacher/students/:id/guidances
func (c *TeacherGuidanceController) Create(ctx echo.Context) error {
	studentID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || studentID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid student id")
	}
	teacherID, ok := middleware.AdminUserIDFromContext(ctx.Request().Context())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	schoolID, err := c.svc.ResolveStudentSchool(uint(studentID))
	if err != nil {
		if errors.Is(err, teacher.ErrGuidanceStudentNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "生徒が見つかりません")
		}
		return httpapi.InternalError(err)
	}
	if err := httpapi.EnsureAdminSchoolAccess(ctx, c.schools, schoolID); err != nil {
		return err
	}

	var req createGuidanceRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}

	view, err := c.svc.Create(teacher.CreateGuidanceInput{
		TeacherUserID:       teacherID,
		StudentUserID:       uint(studentID),
		Kind:                req.Kind,
		Message:             req.Message,
		SuggestedIndustries: req.SuggestedIndustries,
	})
	if err != nil {
		switch {
		case errors.Is(err, teacher.ErrGuidanceInvalidKind),
			errors.Is(err, teacher.ErrGuidanceEmptyMessage),
			errors.Is(err, teacher.ErrGuidanceMessageTooLong):
			return echo.NewHTTPError(http.StatusBadRequest, teacher.FormatCreateError(err))
		default:
			return httpapi.InternalError(err)
		}
	}
	return ctx.JSON(http.StatusCreated, view)
}
