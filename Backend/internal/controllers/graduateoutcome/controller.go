// Package graduateoutcome は学生向けの卒業生の進路実績（#2032）の API。
// 学生は認証（EchoUserAuth）から決め、学校もその学生の登録から決める（クエリで学校を指定させない）。
package graduateoutcome

import (
	"net/http"
	"strconv"
	"strings"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/repositories"
	svc "Backend/internal/services/graduateoutcome"

	"github.com/labstack/echo/v4"
)

type Service interface {
	List(userID uint, f repositories.GraduateOutcomeFilter) (*svc.List, error)
	Company(userID, companyID uint) (*svc.CompanyDetail, error)
}

type Controller struct {
	svc Service
}

func NewController(s Service) *Controller {
	return &Controller{svc: s}
}

func currentUser(ctx echo.Context) (uint, error) {
	id, ok := middleware.UserIDFromContext(ctx.Request().Context())
	if !ok || id == 0 {
		return 0, echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	return id, nil
}

// List は GET /api/graduate-outcomes?year=&industry=
func (c *Controller) List(ctx echo.Context) error {
	userID, err := currentUser(ctx)
	if err != nil {
		return err
	}
	var f repositories.GraduateOutcomeFilter
	if raw := ctx.QueryParam("year"); raw != "" {
		y, err := strconv.Atoi(raw)
		if err != nil || y < 1900 || y > 2100 {
			return echo.NewHTTPError(http.StatusBadRequest, "year が不正です")
		}
		f.Year = y
	}
	f.Industry = strings.TrimSpace(ctx.QueryParam("industry"))
	out, err := c.svc.List(userID, f)
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, out)
}

// Company は GET /api/graduate-outcomes/companies/:id
func (c *Controller) Company(ctx echo.Context) error {
	userID, err := currentUser(ctx)
	if err != nil {
		return err
	}
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "id が不正です")
	}
	out, err := c.svc.Company(userID, uint(id))
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, out)
}
