package controllers

import (
	"Backend/internal/services/admin"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// AdminVectorController は RAG ベクトルDB管理 API の HTTP 入口。
// 外部呼び出しは AdminVectorService に委譲する。
type AdminVectorController struct {
	svc *admin.AdminVectorService
}

func NewAdminVectorController(svc *admin.AdminVectorService) *AdminVectorController {
	if svc == nil {
		svc = admin.NewAdminVectorService()
	}
	return &AdminVectorController{svc: svc}
}

// Status handles GET /api/admin/vector/status?company=
func (c *AdminVectorController) Status(ctx echo.Context) error {
	if !c.svc.Configured() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "RAG_REVIEW_URL is not configured")
	}
	result, err := c.svc.Status(ctx.Request().Context(), ctx.QueryParam("company"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	return ctx.JSON(result.StatusCode, result.Payload)
}

// Reembed handles POST /api/admin/vector/reembed
func (c *AdminVectorController) Reembed(ctx echo.Context) error {
	if !c.svc.Configured() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "RAG_REVIEW_URL is not configured")
	}
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	result, err := c.svc.Reembed(ctx.Request().Context(), body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	return ctx.JSON(result.StatusCode, result.Payload)
}

// Stats handles GET /api/admin/vector/stats - RAG利用統計情報の取得
func (c *AdminVectorController) Stats(ctx echo.Context) error {
	if !c.svc.Configured() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "RAG_REVIEW_URL is not configured")
	}
	result, err := c.svc.Stats(ctx.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	return ctx.JSON(result.StatusCode, result.Payload)
}

// Collections handles GET /api/admin/vector/collections - RAG全コレクション情報の取得
func (c *AdminVectorController) Collections(ctx echo.Context) error {
	if !c.svc.Configured() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "RAG_REVIEW_URL is not configured")
	}
	result, err := c.svc.Collections(ctx.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	return ctx.JSON(result.StatusCode, result.Payload)
}

// 未使用インポート回避（strings は QueryParam 周りで将来使う可能性があったが Status では不要）
var _ = strings.TrimSpace
