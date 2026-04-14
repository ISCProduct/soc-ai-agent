package controllers

import (
	"Backend/internal/services"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// AdminScoreValidationController スコア精度検証・A/Bテスト管理API
type AdminScoreValidationController struct {
	svc *services.ScoreValidationService
}

func NewAdminScoreValidationController(svc *services.ScoreValidationService) *AdminScoreValidationController {
	return &AdminScoreValidationController{svc: svc}
}

// Route /api/admin/score-validation/* のルーティング
func (c *AdminScoreValidationController) Route(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/score-validation")
	path = strings.Trim(path, "/")

	switch {
	case path == "correlation" && r.Method == http.MethodGet:
		c.GetCorrelation(w, r)
	case path == "phase-metrics" && r.Method == http.MethodGet:
		c.GetPhaseMetrics(w, r)
	case path == "calibration" && r.Method == http.MethodGet:
		c.GetCalibration(w, r)
	case path == "calibration/run" && r.Method == http.MethodPost:
		c.RunCalibration(w, r)
	case path == "calibration/history" && r.Method == http.MethodGet:
		c.GetCalibrationHistory(w, r)
	case path == "variants" && r.Method == http.MethodGet:
		c.ListVariants(w, r)
	case path == "variants" && r.Method == http.MethodPost:
		c.CreateVariant(w, r)
	case path == "variants/results" && r.Method == http.MethodGet:
		c.GetVariantResults(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// GetCorrelation GET /api/admin/score-validation/correlation
// カテゴリ別スコアと選考通過率の相関レポート
func (c *AdminScoreValidationController) GetCorrelation(w http.ResponseWriter, r *http.Request) {
	report, err := c.svc.GetCorrelationReport()
	if err != nil {
		writeInternalServerError(w, err)
		return
	}
	writeJSON(w, report)
}

// GetPhaseMetrics GET /api/admin/score-validation/phase-metrics
// フェーズ別予測精度メトリクス
func (c *AdminScoreValidationController) GetPhaseMetrics(w http.ResponseWriter, r *http.Request) {
	report, err := c.svc.GetPhasePrecisionReport()
	if err != nil {
		writeInternalServerError(w, err)
		return
	}
	writeJSON(w, report)
}

// GetCalibration GET /api/admin/score-validation/calibration
// 現在有効なキャリブレーション重みを返す
func (c *AdminScoreValidationController) GetCalibration(w http.ResponseWriter, r *http.Request) {
	weights, err := c.svc.GetCurrentCalibration()
	if err != nil {
		writeInternalServerError(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"weights": weights})
}

// RunCalibration POST /api/admin/score-validation/calibration/run
// 実績データを元にスコアキャリブレーションを実行
func (c *AdminScoreValidationController) RunCalibration(w http.ResponseWriter, r *http.Request) {
	result, err := c.svc.RunCalibration()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, result)
}

// GetCalibrationHistory GET /api/admin/score-validation/calibration/history?limit=10
func (c *AdminScoreValidationController) GetCalibrationHistory(w http.ResponseWriter, r *http.Request) {
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	history, err := c.svc.GetCalibrationHistory(limit)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"history": history})
}

// ListVariants GET /api/admin/score-validation/variants?experiment=xxx
func (c *AdminScoreValidationController) ListVariants(w http.ResponseWriter, r *http.Request) {
	experiments, err := c.svc.ListExperiments()
	if err != nil {
		writeInternalServerError(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"experiments": experiments})
}

// CreateVariant POST /api/admin/score-validation/variants
func (c *AdminScoreValidationController) CreateVariant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExperimentName string  `json:"experiment_name"`
		VariantName    string  `json:"variant_name"`
		Description    string  `json:"description"`
		TrafficRatio   float64 `json:"traffic_ratio"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.ExperimentName == "" || req.VariantName == "" {
		http.Error(w, "experiment_name and variant_name are required", http.StatusBadRequest)
		return
	}
	if req.TrafficRatio <= 0 || req.TrafficRatio > 1 {
		req.TrafficRatio = 0.5
	}

	variant, err := c.svc.CreateVariant(req.ExperimentName, req.VariantName, req.Description, req.TrafficRatio)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, variant)
}

// GetVariantResults GET /api/admin/score-validation/variants/results?experiment=xxx
func (c *AdminScoreValidationController) GetVariantResults(w http.ResponseWriter, r *http.Request) {
	experimentName := r.URL.Query().Get("experiment")
	if experimentName == "" {
		http.Error(w, "experiment query parameter is required", http.StatusBadRequest)
		return
	}
	results, err := c.svc.GetVariantResults(experimentName)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"experiment": experimentName, "results": results})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
