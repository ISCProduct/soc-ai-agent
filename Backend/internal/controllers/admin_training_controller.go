package controllers

import (
	"net/http"
	"strconv"

	"Backend/internal/services/training"

	"github.com/labstack/echo/v4"
)

// AdminTrainingController は学習データのエクスポートを扱う（管理者のみ）。
//
// 候補者の発話と選考結果を含むため、一般ユーザーには開けない。
type AdminTrainingController struct {
	svc *training.Service
}

func NewAdminTrainingController(svc *training.Service) *AdminTrainingController {
	return &AdminTrainingController{svc: svc}
}

// Stats GET /api/admin/training/stats
//
// 学習データがどれだけ揃っているかを返す。ファインチューニングに着手できる量か、
// まだ足りないかを判断するために使う。
func (c *AdminTrainingController) Stats(ctx echo.Context) error {
	stats, err := c.svc.Count(ctx.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "学習データの集計に失敗しました")
	}
	return ctx.JSON(http.StatusOK, stats)
}

// Export GET /api/admin/training/export?limit=100
//
// RAG の /training/export がそのまま受け取れる形（セッションの配列）で返す。
// 教師ラベルは選考結果のみで、AI の発話は含まない。
func (c *AdminTrainingController) Export(ctx echo.Context) error {
	limit := 0
	if v := ctx.QueryParam("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "limit は0以上の整数で指定してください")
		}
		limit = n
	}

	sessions, err := c.svc.Export(ctx.Request().Context(), limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "学習データの取得に失敗しました")
	}
	return ctx.JSON(http.StatusOK, sessions)
}
