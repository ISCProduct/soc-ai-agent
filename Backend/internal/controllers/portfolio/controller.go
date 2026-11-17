// Package portfolio は自己分析ポートフォリオ（#2030）の API。
//
// 学生の個人情報なので、宛先は認証（EchoUserAuth）から決め、user_id をクエリから受け取らない。
package portfolio

import (
	"net/http"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	portfoliosvc "Backend/internal/services/portfolio"

	"github.com/labstack/echo/v4"
)

// Builder はポートフォリオを組み立てる（portfoliosvc.Service が満たす）。
type Builder interface {
	Build(userID uint, sections []string) (*portfoliosvc.Portfolio, error)
}

type Controller struct {
	svc Builder
}

func NewController(svc Builder) *Controller {
	return &Controller{svc: svc}
}

// Get は本人のポートフォリオを返す。GET /api/portfolio?sections=diagnosis,interview,es,assessment
func (c *Controller) Get(ctx echo.Context) error {
	userID, ok := middleware.UserIDFromContext(ctx.Request().Context())
	if !ok || userID == 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	sections := portfoliosvc.ParseSections(ctx.QueryParam("sections"))
	if len(sections) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "出力する内容を1つ以上選んでください")
	}
	p, err := c.svc.Build(userID, sections)
	if err != nil {
		return httpapi.InternalError(err)
	}
	// 個人情報を含むので、共有のキャッシュに残さない
	ctx.Response().Header().Set("Cache-Control", "no-store")
	return ctx.JSON(http.StatusOK, p)
}
