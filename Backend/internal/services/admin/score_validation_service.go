package admin

import (
	"Backend/internal/models"
	"Backend/internal/repositories"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// ErrInsufficientSamples サンプル数不足でキャリブレーションが実行できない場合
var ErrInsufficientSamples = errors.New("キャリブレーションに必要なサンプル数が不足しています（確定結果がカテゴリあたり通過・不採用各2件以上かつ合計10件以上必要）")

// ErrCalibrationUnchanged 算出重みが現行と実質同じで保存をスキップした場合
var ErrCalibrationUnchanged = errors.New("キャリブレーション重みに有意な変化がないため保存をスキップしました")

const (
	calibrationShrinkK      = 20.0 // サンプルが少ないほど 1.0 へ縮む
	calibrationSignalScale  = 50.0 // Signal ±50 → 生重み 0..2
	calibrationMaxStep      = 0.3  // 1回の自動適用で動かす上限
	calibrationMinChange    = 0.05 // これ未満の差分は「変化なし」
	autoCalibrationInterval = 24 * time.Hour
	calibrationWeightMin    = 0.1
	calibrationWeightMax    = 3.0
)

// ScoreValidationService チャット分析スコアの精度検証・改善サービス
type ScoreValidationService struct {
	repo *repositories.ScoreValidationRepository
}

func NewScoreValidationService(repo *repositories.ScoreValidationRepository) *ScoreValidationService {
	return &ScoreValidationService{repo: repo}
}

// ── 相関分析レポート ──────────────────────────────────────────────────────────

// CorrelationReport 相関分析レポート
type CorrelationReport struct {
	Rows          []repositories.CategoryCorrelationRow `json:"rows"`
	TopCorrelated []string                              `json:"top_correlated"` // 通過率との相関が高いカテゴリ
	LowCorrelated []string                              `json:"low_correlated"` // 相関が低いカテゴリ（改善候補）
	TotalSamples  int                                   `json:"total_samples"`
}

// GetCorrelationReport スコアと選考通過率の相関レポートを生成する
func (s *ScoreValidationService) GetCorrelationReport() (*CorrelationReport, error) {
	rows, err := s.repo.GetCategoryPassRateCorrelation()
	if err != nil {
		return nil, fmt.Errorf("相関集計エラー: %w", err)
	}

	// カテゴリ別に通過率の分散を計算して相関の高低を判定
	type catStat struct {
		passRates []float64
		totalN    int
	}
	catMap := map[string]*catStat{}
	for _, row := range rows {
		if _, ok := catMap[row.Category]; !ok {
			catMap[row.Category] = &catStat{}
		}
		catMap[row.Category].passRates = append(catMap[row.Category].passRates, row.PassRate)
		catMap[row.Category].totalN += row.TotalCount
	}

	// 分散（スコアが高いほど通過率が上がるか）を計算
	type catVariance struct {
		category string
		variance float64
	}
	variances := make([]catVariance, 0, len(catMap))
	for cat, stat := range catMap {
		if len(stat.passRates) < 2 {
			continue
		}
		mean := 0.0
		for _, r := range stat.passRates {
			mean += r
		}
		mean /= float64(len(stat.passRates))
		variance := 0.0
		for _, r := range stat.passRates {
			variance += (r - mean) * (r - mean)
		}
		variance /= float64(len(stat.passRates))
		variances = append(variances, catVariance{cat, variance})
	}

	// 分散が大きい（スコア帯で通過率の差が大きい）カテゴリを "相関高" とみなす
	top, low := []string{}, []string{}
	threshold := 100.0 // 分散閾値（調整可能）
	for _, v := range variances {
		if v.variance >= threshold {
			top = append(top, v.category)
		} else {
			low = append(low, v.category)
		}
	}

	totalSamples := 0
	for _, stat := range catMap {
		totalSamples += stat.totalN
	}

	return &CorrelationReport{
		Rows:          rows,
		TopCorrelated: top,
		LowCorrelated: low,
		TotalSamples:  totalSamples,
	}, nil
}

// ── フェーズ精度メトリクス ────────────────────────────────────────────────────

// PhasePrecisionReport フェーズ別精度メトリクスレポート
type PhasePrecisionReport struct {
	Phases      []repositories.PhasePrecisionRow `json:"phases"`
	OverallPass float64                          `json:"overall_pass_rate"`
}

func (s *ScoreValidationService) GetPhasePrecisionReport() (*PhasePrecisionReport, error) {
	phases, err := s.repo.GetPhasePrecisionMetrics()
	if err != nil {
		return nil, fmt.Errorf("フェーズメトリクス集計エラー: %w", err)
	}

	totalPass, totalSessions := 0.0, 0
	for _, p := range phases {
		totalPass += p.PassRate
		totalSessions++
	}
	overallPass := 0.0
	if totalSessions > 0 {
		overallPass = math.Round(totalPass/float64(totalSessions)*10) / 10
	}

	return &PhasePrecisionReport{
		Phases:      phases,
		OverallPass: overallPass,
	}, nil
}

// ── A/Bテスト バリアント管理 ─────────────────────────────────────────────────

// CreateVariant 新しい質問バリアントを登録する
func (s *ScoreValidationService) CreateVariant(experimentName, variantName, description string, trafficRatio float64) (*models.QuestionVariant, error) {
	v := &models.QuestionVariant{
		ExperimentName: experimentName,
		VariantName:    variantName,
		Description:    description,
		IsActive:       true,
		TrafficRatio:   trafficRatio,
	}
	if err := s.repo.CreateVariant(v); err != nil {
		return nil, err
	}
	return v, nil
}

// AssignVariant セッションにバリアントをランダム割り当てする
func (s *ScoreValidationService) AssignVariant(userID uint, sessionID, experimentName string) (*models.VariantAssignment, error) {
	// 既存割り当て確認
	existing, err := s.repo.FindAssignment(userID, sessionID)
	if err == nil {
		return existing, nil
	}

	variants, err := s.repo.ListActiveVariants(experimentName)
	if err != nil || len(variants) == 0 {
		return nil, fmt.Errorf("アクティブなバリアントが見つかりません: %s", experimentName)
	}

	// traffic_ratio に基づく重み付きランダム選択
	r := rand.Float64()
	cumulative := 0.0
	selected := variants[0]
	for _, v := range variants {
		cumulative += v.TrafficRatio
		if r <= cumulative {
			selected = v
			break
		}
	}

	assignment := &models.VariantAssignment{
		UserID:          userID,
		SessionID:       sessionID,
		VariantID:       selected.ID,
		ExperimentName:  experimentName,
		AssignedVariant: selected.VariantName,
	}
	if err := s.repo.AssignVariant(assignment); err != nil {
		return nil, err
	}
	return assignment, nil
}

// GetVariantResults 実験バリアント別の通過率レポートを返す
func (s *ScoreValidationService) GetVariantResults(experimentName string) ([]repositories.VariantResultRow, error) {
	return s.repo.GetVariantResults(experimentName)
}

// ListExperiments 実験名一覧を返す
func (s *ScoreValidationService) ListExperiments() ([]string, error) {
	return s.repo.ListAllExperiments()
}

// ListAllVariants 全実験・全バリアントの詳細一覧を返す（管理画面の一覧表示用）
func (s *ScoreValidationService) ListAllVariants() ([]models.QuestionVariant, error) {
	return s.repo.ListAllVariants()
}

// ── スコアキャリブレーション ──────────────────────────────────────────────────

// CalibrationResult キャリブレーション実行結果
type CalibrationResult struct {
	Version int                             `json:"version"`
	Weights []models.ScoreCalibrationWeight `json:"weights"`
	Message string                          `json:"message"`
}

// correlationApprox は重みから相関係数の簡易近似を返す。
// weight=1.0 が中立、1.0超が正の信号、1.0未満が負の信号。
func correlationApprox(weight float64) float64 {
	return math.Max(-1.0, math.Min(weight-1.0, 1.0))
}

// WeightFromPassFailSignal は通過/不採用のスコア差から軸重みを作る。
// Signal = avg(score|pass) - avg(score|fail)。サンプルが少ないほど 1.0 へ縮む。
func WeightFromPassFailSignal(signal float64, sampleN int) float64 {
	raw := 1.0 + signal/calibrationSignalScale
	shrink := 1.0
	if sampleN > 0 {
		shrink = float64(sampleN) / (float64(sampleN) + calibrationShrinkK)
	}
	w := 1.0 + (raw-1.0)*shrink
	return clampCalibrationWeight(w)
}

func clampCalibrationWeight(w float64) float64 {
	if w < calibrationWeightMin {
		return calibrationWeightMin
	}
	if w > calibrationWeightMax {
		return calibrationWeightMax
	}
	return w
}

func stepToward(prev, next, maxStep float64) float64 {
	d := next - prev
	if d > maxStep {
		return prev + maxStep
	}
	if d < -maxStep {
		return prev - maxStep
	}
	return next
}

// RunCalibration 実績データからスコア重みを再計算して保存する。
// 通過側と不採用側の応募時点スコア差（Signal）を軸の重要度にし、
// マッチングのカテゴリ重みとして保存する。
func (s *ScoreValidationService) RunCalibration() (*CalibrationResult, error) {
	return s.runCalibration(true)
}

// AutoCalibrateIfDue は間隔が空いていれば自動キャリブレーションする。
// サンプル不足・変化なしはエラーにせずスキップ（定期ジョブ用）。
func (s *ScoreValidationService) AutoCalibrateIfDue(now time.Time) (*CalibrationResult, error) {
	last, err := s.repo.LastCalibrationAt()
	if err != nil {
		return nil, err
	}
	if last != nil && now.Sub(*last) < autoCalibrationInterval {
		return nil, nil
	}
	result, err := s.runCalibration(true)
	if err != nil {
		if errors.Is(err, ErrInsufficientSamples) || errors.Is(err, ErrCalibrationUnchanged) {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}

func (s *ScoreValidationService) runCalibration(stepCap bool) (*CalibrationResult, error) {
	stats, err := s.repo.GetCategoryPassStats()
	if err != nil {
		return nil, fmt.Errorf("統計取得エラー: %w", err)
	}
	if len(stats) == 0 {
		return nil, ErrInsufficientSamples
	}

	prevByCat := map[string]float64{}
	if prev, err := s.repo.GetLatestCalibrationWeights(); err == nil {
		for _, w := range prev {
			prevByCat[w.Category] = w.Weight
		}
	}

	nextVersion, err := s.repo.GetNextVersion()
	if err != nil {
		return nil, err
	}

	weights := make([]models.ScoreCalibrationWeight, 0, len(stats))
	maxDelta := 0.0
	for _, stat := range stats {
		target := WeightFromPassFailSignal(stat.Signal, stat.SampleN)
		prev := 1.0
		if p, ok := prevByCat[stat.Category]; ok {
			prev = p
		}
		weight := target
		if stepCap {
			weight = stepToward(prev, target, calibrationMaxStep)
		}
		weight = math.Round(clampCalibrationWeight(weight)*100) / 100
		if d := math.Abs(weight - prev); d > maxDelta {
			maxDelta = d
		}
		weights = append(weights, models.ScoreCalibrationWeight{
			Category:    stat.Category,
			Version:     nextVersion,
			Weight:      weight,
			SampleCount: stat.SampleN,
			PassRate:    math.Round(stat.PassRate*10) / 10,
			Correlation: math.Round(correlationApprox(weight)*100) / 100,
			IsActive:    true,
		})
	}

	if maxDelta < calibrationMinChange {
		return nil, ErrCalibrationUnchanged
	}

	if err := s.repo.SaveCalibrationWeights(weights); err != nil {
		return nil, fmt.Errorf("重み保存エラー: %w", err)
	}

	return &CalibrationResult{
		Version: nextVersion,
		Weights: weights,
		Message: fmt.Sprintf("キャリブレーション完了: %d カテゴリ、確定サンプル合計 %d 件（通過/不採用スコア差ベース）",
			len(weights), totalSampleCount(stats)),
	}, nil
}

// GetCurrentCalibration 現在有効なキャリブレーション重みを返す
func (s *ScoreValidationService) GetCurrentCalibration() ([]models.ScoreCalibrationWeight, error) {
	return s.repo.GetLatestCalibrationWeights()
}

// GetCalibrationHistory キャリブレーション履歴を返す
func (s *ScoreValidationService) GetCalibrationHistory(limit int) ([]models.ScoreCalibrationWeight, error) {
	if limit <= 0 {
		limit = 10
	}
	return s.repo.ListCalibrationHistory(limit)
}

func totalSampleCount(stats []repositories.CategoryPassStats) int {
	total := 0
	for _, s := range stats {
		total += s.SampleN
	}
	return total
}
