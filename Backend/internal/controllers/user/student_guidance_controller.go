package user

import (
	"errors"
	"net/http"
	"strconv"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/services/teacher"

	"github.com/labstack/echo/v4"
)

type StudentGuidanceController struct {
	svc *teacher.GuidanceService
}

func NewStudentGuidanceController(svc *teacher.GuidanceService) *StudentGuidanceController {
	return &StudentGuidanceController{svc: svc}
}

// List GET /api/user/guidances
func (c *StudentGuidanceController) List(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	rows, err := c.svc.ListActiveForStudent(userID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"guidances": rows})
}

// Dismiss POST /api/user/guidances/:id/dismiss
func (c *StudentGuidanceController) Dismiss(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	if err := c.svc.Dismiss(userID, uint(id)); err != nil {
		if errors.Is(err, teacher.ErrGuidanceStudentNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "案内が見つかりません")
		}
		return httpapi.InternalError(err)
	}
	return ctx.NoContent(http.StatusNoContent)
}
