package controllers

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// Names GET /api/admin/companies/names
// アクティブな企業の {id, name} 一覧を返す。q による部分一致検索をサポート。
func (c *AdminCompanyController) Names(ctx echo.Context) error {
	q := strings.TrimSpace(ctx.QueryParam("q"))
	names, err := c.repo.FindAllActiveNames(q)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch company names")
	}
	return ctx.JSON(http.StatusOK, names)
}
