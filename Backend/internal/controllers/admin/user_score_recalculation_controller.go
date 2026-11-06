package admin

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"Backend/internal/controllers/httpapi"
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
		log.Printf("[UserScoreRecalculation] preview failed: %v", err)
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
		log.Printf("[UserScoreRecalculation] start failed: %v", err)
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
	if err != nil {
		return recalculationJobError(id, err)
	}
	return ctx.JSON(http.StatusOK, job)
}

func (c *UserScoreRecalculationController) RunBatch(ctx echo.Context) error {
	id, err := parseRecalculationJobID(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "job_id が不正です")
	}
	job, err := c.service.RunBatch(ctx.Request().Context(), id)
	if err != nil {
		return recalculationJobError(id, err)
	}
	return ctx.JSON(http.StatusOK, job)
}

func (c *UserScoreRecalculationController) Pause(ctx echo.Context) error {
	return c.setJobStatus(ctx, c.service.Pause)
}

func (c *UserScoreRecalculationController) Resume(ctx echo.Context) error {
	return c.setJobStatus(ctx, c.service.Resume)
}

// DryRun は POST /api/admin/user-score-recalculation/dry-run。保存せずに再計算後の値を返す。
func (c *UserScoreRecalculationController) DryRun(ctx echo.Context) error {
	limit := httpapi.LimitQuery(ctx, "limit", 3)
	result, err := c.service.DryRun(ctx.Request().Context(), limit)
	if err != nil {
		log.Printf("[UserScoreRecalculation] dry-run failed: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "ドライランに失敗しました")
	}
	return ctx.JSON(http.StatusOK, result)
}

// Rollback は POST /api/admin/user-score-recalculation/jobs/:job_id/rollback。
func (c *UserScoreRecalculationController) Rollback(ctx echo.Context) error {
	id, err := parseRecalculationJobID(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "job_id が不正です")
	}
	result, err := c.service.Rollback(ctx.Request().Context(), id)
	if err != nil {
		return recalculationJobError(id, err)
	}
	return ctx.JSON(http.StatusOK, result)
}

func (c *UserScoreRecalculationController) Cancel(ctx echo.Context) error {
	return c.setJobStatus(ctx, c.service.Cancel)
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
	if err != nil {
		return recalculationJobError(id, err)
	}
	return ctx.JSON(http.StatusOK, job)
}

// recalculationJobError はエラーを HTTP の応答へ変換する。
// 状態の競合だけを 409 にし、それ以外は本文をログに残して 500 を返す。
func recalculationJobError(id uint, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "再計算ジョブが見つかりません")
	}
	if message, ok := adminservice.RecalculationConflictMessage(err); ok {
		return echo.NewHTTPError(http.StatusConflict, message)
	}
	log.Printf("[UserScoreRecalculation] job=%d request failed: %v", id, err)
	return echo.NewHTTPError(http.StatusInternalServerError, "再計算ジョブの処理に失敗しました")
}

func parseRecalculationJobID(ctx echo.Context) (uint, error) {
	id, err := strconv.ParseUint(ctx.Param("job_id"), 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid job id")
	}
	return uint(id), nil
}
