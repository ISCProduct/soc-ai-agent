package controllers

import (
	"Backend/internal/services"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// AdminScoreValidationController スコア精度検証・A/Bテスト管理API
type AdminScoreValidationController struct {
	svc *services.ScoreValidationService
}

func NewAdminScoreValidationController(svc *services.ScoreValidationService) *AdminScoreValidationController {
	return &AdminScoreValidationController{svc: svc}
}

// GetCorrelation GET /api/admin/score-validation/correlation
// カテゴリ別スコアと選考通過率の相関レポート
func (c *AdminScoreValidationController) GetCorrelation(ctx echo.Context) error {
	report, err := c.svc.GetCorrelationReport()
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, report)
}

// GetPhaseMetrics GET /api/admin/score-validation/phase-metrics
// フェーズ別予測精度メトリクス
func (c *AdminScoreValidationController) GetPhaseMetrics(ctx echo.Context) error {
	report, err := c.svc.GetPhasePrecisionReport()
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, report)
}

// GetCalibration GET /api/admin/score-validation/calibration
// 現在有効なキャリブレーション重みを返す
func (c *AdminScoreValidationController) GetCalibration(ctx echo.Context) error {
	weights, err := c.svc.GetCurrentCalibration()
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"weights": weights})
}

// RunCalibration POST /api/admin/score-validation/calibration/run
// 実績データを元にスコアキャリブレーションを実行
func (c *AdminScoreValidationController) RunCalibration(ctx echo.Context) error {
	result, err := c.svc.RunCalibration()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusCreated, result)
}

// GetCalibrationHistory GET /api/admin/score-validation/calibration/history?limit=10
func (c *AdminScoreValidationController) GetCalibrationHistory(ctx echo.Context) error {
	limit := 10
	if l := ctx.QueryParam("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	history, err := c.svc.GetCalibrationHistory(limit)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"history": history})
}

// ListVariants GET /api/admin/score-validation/variants
func (c *AdminScoreValidationController) ListVariants(ctx echo.Context) error {
	experiments, err := c.svc.ListExperiments()
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"experiments": experiments})
}

// CreateVariant POST /api/admin/score-validation/variants
func (c *AdminScoreValidationController) CreateVariant(ctx echo.Context) error {
	var req struct {
		ExperimentName string  `json:"experiment_name"`
		VariantName    string  `json:"variant_name"`
		Description    string  `json:"description"`
		TrafficRatio   float64 `json:"traffic_ratio"`
	}
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.ExperimentName == "" || req.VariantName == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "experiment_name and variant_name are required")
	}
	if req.TrafficRatio <= 0 || req.TrafficRatio > 1 {
		req.TrafficRatio = 0.5
	}

	variant, err := c.svc.CreateVariant(req.ExperimentName, req.VariantName, req.Description, req.TrafficRatio)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusCreated, variant)
}

// GetVariantResults GET /api/admin/score-validation/variants/results?experiment=xxx
func (c *AdminScoreValidationController) GetVariantResults(ctx echo.Context) error {
	experimentName := ctx.QueryParam("experiment")
	if experimentName == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "experiment query parameter is required")
	}
	results, err := c.svc.GetVariantResults(experimentName)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"experiment": experimentName, "results": results})
}
