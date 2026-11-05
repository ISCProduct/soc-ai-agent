package admin

import (
	"errors"
	"net/http"
	"strconv"

	"Backend/internal/models"
	adminservice "Backend/internal/services/admin"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type UserScoreRecalculationController struct {
	service *adminservice.UserScoreRecalculationService
}

func NewUserScoreRecalculationController(service *adminservice.UserScoreRecalculationService) *UserScoreRecalculationController {
	return &UserScoreRecalculationController{service: service}
}

func (c *UserScoreRecalculationController) Preview(ctx echo.Context) error {
	result, err := c.service.Preview()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "スコア再計算の見積もりに失敗しました")
	}
	return ctx.JSON(http.StatusOK, result)
}

func (c *UserScoreRecalculationController) Start(ctx echo.Context) error {
	job, err := c.service.Start()
	if err != nil {
		if adminservice.IsRecalculationJobAlreadyRunning(err) {
			return echo.NewHTTPError(http.StatusConflict, "実行中または一時停止中の再計算ジョブがあります")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "再計算ジョブを開始できませんでした")
	}
	return ctx.JSON(http.StatusCreated, job)
}

func (c *UserScoreRecalculationController) GetJob(ctx echo.Context) error {
	id, err := parseRecalculationJobID(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "job_id が不正です")
	}
	job, err := c.service.GetJob(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "再計算ジョブが見つかりません")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "再計算ジョブを取得できませんでした")
	}
	return ctx.JSON(http.StatusOK, job)
}

func (c *UserScoreRecalculationController) RunBatch(ctx echo.Context) error {
	id, err := parseRecalculationJobID(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "job_id が不正です")
	}
	job, err := c.service.RunBatch(ctx.Request().Context(), id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "再計算ジョブが見つかりません")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	}
	return ctx.JSON(http.StatusOK, job)
}

func (c *UserScoreRecalculationController) Pause(ctx echo.Context) error {
	return c.setJobStatus(ctx, c.service.Pause)
}

func (c *UserScoreRecalculationController) Resume(ctx echo.Context) error {
	return c.setJobStatus(ctx, c.service.Resume)
}

func (c *UserScoreRecalculationController) setJobStatus(
	ctx echo.Context,
	update func(uint) (*models.UserScoreRecalculationJob, error),
) error {
	id, err := parseRecalculationJobID(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "job_id が不正です")
	}
	job, err := update(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "再計算ジョブが見つかりません")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	}
	return ctx.JSON(http.StatusOK, job)
}

func parseRecalculationJobID(ctx echo.Context) (uint, error) {
	id, err := strconv.ParseUint(ctx.Param("job_id"), 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid job id")
	}
	return uint(id), nil
}
