package repositories

import (
	"Backend/internal/models"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"gorm.io/gorm"
)

type ScoreValidationRepository struct {
	db *gorm.DB
}

func NewScoreValidationRepository(db *gorm.DB) *ScoreValidationRepository {
	return &ScoreValidationRepository{db: db}
}

func flywheelPassedSQL(query string) string {
	return fmt.Sprintf(query, models.FlywheelPassedStatusSQLIn())
}

// ── 相関分析クエリ ────────────────────────────────────────────────────────────

// CategoryCorrelationRow カテゴリ別スコア vs 通過率の相関行データ
type CategoryCorrelationRow struct {
	Category   string  `json:"category"`
	ScoreBand  string  `json:"score_band"` // "0-20", "21-40", "41-60", "61-80", "81-100"
	TotalCount int     `json:"total_count"`
	PassCount  int     `json:"pass_count"` // 書類通過以降（models.FlywheelPassedStatuses + 旧 interview）
	PassRate   float64 `json:"pass_rate"`  // 0-100 のパーセント値
	AvgScore   float64 `json:"avg_score"`
}

// GetCategoryPassRateCorrelation カテゴリ別スコア帯と選考通過率の相関を集計する
// 通過判定: models.FlywheelPassedStatusSQLIn（現行 ValidStatuses の書類通過以降 + 旧 interview）
func (r *ScoreValidationRepository) GetCategoryPassRateCorrelation() ([]CategoryCorrelationRow, error) {
	type rawRow struct {
		Category   string
		ScoreBand  int
		TotalCount int
		PassCount  int
		AvgScore   float64
	}

	// スコアを20点刻みのバンドに丸める（0→0, 21→1, ... ）
	rows := []rawRow{}
	err := r.db.Raw(flywheelPassedSQL(`
		SELECT
			uws.weight_category AS category,
			FLOOR(uws.score / 20) AS score_band,
			COUNT(DISTINCT uas.id) AS total_count,
			SUM(CASE WHEN uas.status IN (%[1]s) THEN 1 ELSE 0 END) AS pass_count,
			AVG(uws.score) AS avg_score
		FROM user_weight_scores uws
		INNER JOIN user_application_statuses uas ON uas.user_id = uws.user_id
		WHERE uas.status != ''
		GROUP BY uws.weight_category, FLOOR(uws.score / 20)
		ORDER BY uws.weight_category, score_band
	`)).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	bandLabel := func(b int) string {
		lo := b * 20
		hi := lo + 20
		if hi > 100 {
			hi = 100
		}
		return fmt.Sprintf("%d-%d", lo, hi)
	}

	result := make([]CategoryCorrelationRow, 0, len(rows))
	for _, row := range rows {
		passRate := 0.0
		if row.TotalCount > 0 {
			passRate = float64(row.PassCount) / float64(row.TotalCount) * 100
		}
		result = append(result, CategoryCorrelationRow{
			Category:   row.Category,
			ScoreBand:  bandLabel(row.ScoreBand),
			TotalCount: row.TotalCount,
			PassCount:  row.PassCount,
			PassRate:   math.Round(passRate*10) / 10,
			AvgScore:   math.Round(row.AvgScore*10) / 10,
		})
	}
	return result, nil
}

// PhasePrecisionRow フェーズ別精度メトリクス行
type PhasePrecisionRow struct {
	PhaseName     string  `json:"phase_name"`
	SessionCount  int     `json:"session_count"`
	AvgCompletion float64 `json:"avg_completion"` // 0-100
	PassCount     int     `json:"pass_count"`
	PassRate      float64 `json:"pass_rate"` // 0-100 のパーセント値
}

// GetPhasePrecisionMetrics フェーズ別の完了率・通過率相関を集計する
func (r *ScoreValidationRepository) GetPhasePrecisionMetrics() ([]PhasePrecisionRow, error) {
	rows := []PhasePrecisionRow{}
	err := r.db.Raw(flywheelPassedSQL(`
		SELECT
			ap.phase_name,
			COUNT(DISTINCT uap.session_id) AS session_count,
			AVG(uap.completion_score) AS avg_completion,
			COUNT(DISTINCT CASE WHEN uas.status IN (%[1]s) THEN uas.user_id END) AS pass_count,
			CASE
				WHEN COUNT(DISTINCT uap.user_id) = 0 THEN 0
				ELSE CAST(COUNT(DISTINCT CASE WHEN uas.status IN (%[1]s) THEN uas.user_id END) AS FLOAT) /
					COUNT(DISTINCT uap.user_id) * 100
			END AS pass_rate
		FROM user_analysis_progress uap
		INNER JOIN analysis_phases ap ON ap.id = uap.phase_id
		LEFT JOIN user_application_statuses uas ON uas.user_id = uap.user_id
		GROUP BY ap.phase_name, ap.phase_order
		ORDER BY ap.phase_order
	`)).Scan(&rows).Error
	return rows, err
}

// ── A/Bテスト バリアント管理 ─────────────────────────────────────────────────

func (r *ScoreValidationRepository) CreateVariant(v *models.QuestionVariant) error {
	return r.db.Create(v).Error
}

func (r *ScoreValidationRepository) ListActiveVariants(experimentName string) ([]models.QuestionVariant, error) {
	var variants []models.QuestionVariant
	err := r.db.Where("experiment_name = ? AND is_active = ?", experimentName, true).
		Find(&variants).Error
	return variants, err
}

func (r *ScoreValidationRepository) ListAllExperiments() ([]string, error) {
	var names []string
	err := r.db.Model(&models.QuestionVariant{}).
		Distinct("experiment_name").
		Pluck("experiment_name", &names).Error
	return names, err
}

func (r *ScoreValidationRepository) AssignVariant(a *models.VariantAssignment) error {
	return r.db.Create(a).Error
}

func (r *ScoreValidationRepository) FindAssignment(userID uint, sessionID string) (*models.VariantAssignment, error) {
	var a models.VariantAssignment
	err := r.db.Where("user_id = ? AND session_id = ?", userID, sessionID).
		Preload("Variant").First(&a).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// VariantResultRow A/Bテスト結果比較行
type VariantResultRow struct {
	ExperimentName string  `json:"experiment_name"`
	VariantName    string  `json:"variant_name"`
	SampleCount    int     `json:"sample_count"`
	PassCount      int     `json:"pass_count"`
	PassRate       float64 `json:"pass_rate"` // 0-100 のパーセント値
	AvgScore       float64 `json:"avg_score"` // 割り当てセッションの平均完了スコア（0-100）
}

// GetVariantResults 実験バリアント別の通過率・平均完了スコアを集計する
func (r *ScoreValidationRepository) GetVariantResults(experimentName string) ([]VariantResultRow, error) {
	rows := []VariantResultRow{}
	err := r.db.Raw(flywheelPassedSQL(`
		SELECT
			va.experiment_name,
			va.assigned_variant AS variant_name,
			COUNT(DISTINCT va.session_id) AS sample_count,
			COUNT(DISTINCT CASE WHEN uas.status IN (%[1]s) THEN uas.user_id END) AS pass_count,
			CASE
				WHEN COUNT(DISTINCT va.user_id) = 0 THEN 0
				ELSE CAST(COUNT(DISTINCT CASE WHEN uas.status IN (%[1]s) THEN uas.user_id END) AS FLOAT) /
					COUNT(DISTINCT va.user_id) * 100
			END AS pass_rate,
			COALESCE(AVG(uap.completion_score), 0) AS avg_score
		FROM variant_assignments va
		LEFT JOIN user_application_statuses uas ON uas.user_id = va.user_id
		LEFT JOIN user_analysis_progress uap ON uap.session_id = va.session_id
		WHERE va.experiment_name = ?
		GROUP BY va.experiment_name, va.assigned_variant
	`), experimentName).Scan(&rows).Error
	return rows, err
}

// ListAllVariants 全実験・全バリアントの詳細一覧を返す（管理画面の一覧表示用）。
func (r *ScoreValidationRepository) ListAllVariants() ([]models.QuestionVariant, error) {
	var variants []models.QuestionVariant
	err := r.db.Order("experiment_name ASC, created_at ASC").Find(&variants).Error
	return variants, err
}

// ── キャリブレーション ────────────────────────────────────────────────────────

func (r *ScoreValidationRepository) GetLatestCalibrationWeights() ([]models.ScoreCalibrationWeight, error) {
	var weights []models.ScoreCalibrationWeight
	err := r.db.Where("is_active = ?", true).Find(&weights).Error
	return weights, err
}

func (r *ScoreValidationRepository) SaveCalibrationWeights(weights []models.ScoreCalibrationWeight) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.ScoreCalibrationWeight{}).
			Where("is_active = ?", true).
			Update("is_active", false).Error; err != nil {
			return err
		}
		return tx.Create(&weights).Error
	})
}

// CategoryPassStats キャリブレーション計算用の生データ。
// Signal は「通過側の平均特徴量 − 不採用側の平均特徴量」（特徴量は応募時点スコア）。
// 正ならそのカテゴリの高スコアが通過と共起しやすい。
type CategoryPassStats struct {
	Category string
	AvgScore float64
	PassRate float64
	SampleN  int
	Signal   float64
	PassN    int
	FailN    int
}

// calibrationMinDecided はカテゴリあたりの確定結果（通過+不採用）の下限。
const calibrationMinDecided = 10

// calibrationMinPerClass は通過・不採用それぞれに必要な下限（片方に偏ると信号が取れない）。
const calibrationMinPerClass = 2

func (r *ScoreValidationRepository) GetCategoryPassStats() ([]CategoryPassStats, error) {
	// 応募時点スナップショット＋確定結果（通過/不採用）を優先。
	// 無い／足りない場合のみ従来の user_id 結合へフォールバックする。
	if rows, err := r.getCategoryPassStatsFromSnapshots(); err != nil {
		return nil, err
	} else if len(rows) > 0 {
		return rows, nil
	}
	return r.getCategoryPassStatsLegacy()
}

func parseScoreSnapshot(raw string) (map[string]float64, bool) {
	snap := map[string]float64{}
	if err := json.Unmarshal([]byte(raw), &snap); err == nil {
		return snap, len(snap) > 0
	}
	intSnap := map[string]int{}
	if err := json.Unmarshal([]byte(raw), &intSnap); err != nil {
		return nil, false
	}
	for k, v := range intSnap {
		snap[k] = float64(v)
	}
	return snap, len(snap) > 0
}

func isCalibrationPassStatus(status string) bool {
	for _, s := range models.FlywheelPassedStatusFilter() {
		if s == status {
			return true
		}
	}
	return false
}

func isCalibrationFailStatus(status string) bool {
	return status == "rejected"
}

func (r *ScoreValidationRepository) getCategoryPassStatsFromSnapshots() ([]CategoryPassStats, error) {
	type appRow struct {
		Status        string
		ScoreSnapshot string
	}
	var apps []appRow
	// 確定結果だけ使う。進行中・辞退は通過でも不採用でもないので混ぜない。
	if err := r.db.Model(&models.UserApplicationStatus{}).
		Select("status, score_snapshot").
		Where("score_snapshot IS NOT NULL AND score_snapshot != ''").
		Where("status IN ? OR status = ?", models.FlywheelPassedStatusFilter(), "rejected").
		Find(&apps).Error; err != nil {
		return nil, err
	}
	if len(apps) == 0 {
		return nil, nil
	}

	type agg struct {
		sumAll  float64
		n       int
		passSum float64
		passN   int
		failSum float64
		failN   int
	}
	byCat := map[string]*agg{}
	for _, app := range apps {
		snap, ok := parseScoreSnapshot(app.ScoreSnapshot)
		if !ok {
			continue
		}
		pass := isCalibrationPassStatus(app.Status)
		fail := isCalibrationFailStatus(app.Status)
		if !pass && !fail {
			continue
		}
		for cat, score := range snap {
			a := byCat[cat]
			if a == nil {
				a = &agg{}
				byCat[cat] = a
			}
			a.sumAll += score
			a.n++
			if pass {
				a.passSum += score
				a.passN++
			} else {
				a.failSum += score
				a.failN++
			}
		}
	}

	out := make([]CategoryPassStats, 0, len(byCat))
	for cat, a := range byCat {
		if a.n < calibrationMinDecided || a.passN < calibrationMinPerClass || a.failN < calibrationMinPerClass {
			continue
		}
		passAvg := a.passSum / float64(a.passN)
		failAvg := a.failSum / float64(a.failN)
		out = append(out, CategoryPassStats{
			Category: cat,
			AvgScore: a.sumAll / float64(a.n),
			PassRate: float64(a.passN) / float64(a.n) * 100,
			SampleN:  a.n,
			Signal:   passAvg - failAvg,
			PassN:    a.passN,
			FailN:    a.failN,
		})
	}
	return out, nil
}

func (r *ScoreValidationRepository) getCategoryPassStatsLegacy() ([]CategoryPassStats, error) {
	// スナップショットが無い古いデータの最終手段。通過率比しか取れないので Signal=0。
	rows := []CategoryPassStats{}
	err := r.db.Raw(flywheelPassedSQL(`
		SELECT
			uws.weight_category AS category,
			AVG(uws.score) AS avg_score,
			CASE
				WHEN COUNT(DISTINCT uas.id) = 0 THEN 0
				ELSE CAST(SUM(CASE WHEN uas.status IN (%[1]s) THEN 1 ELSE 0 END) AS FLOAT) /
					COUNT(DISTINCT uas.id) * 100
			END AS pass_rate,
			COUNT(DISTINCT uas.id) AS sample_n
		FROM user_weight_scores uws
		INNER JOIN user_application_statuses uas ON uas.user_id = uws.user_id
		WHERE uas.status IN (%[1]s) OR uas.status = 'rejected'
		GROUP BY uws.weight_category
		HAVING COUNT(DISTINCT uas.id) >= 10
	`)).Scan(&rows).Error
	return rows, err
}

// ── キャリブレーション重みの取得（バージョン管理） ──────────────────────────

func (r *ScoreValidationRepository) GetNextVersion() (int, error) {
	var maxVersion int
	err := r.db.Model(&models.ScoreCalibrationWeight{}).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion).Error
	return maxVersion + 1, err
}

// ListCalibrationHistory バージョン一覧（降順）
func (r *ScoreValidationRepository) ListCalibrationHistory(limit int) ([]models.ScoreCalibrationWeight, error) {
	var weights []models.ScoreCalibrationWeight
	err := r.db.Order("version DESC, created_at DESC").
		Limit(limit).
		Find(&weights).Error
	return weights, err
}

// LastCalibrationAt 最後にキャリブレーションが実行された日時。未実行なら nil, nil。
func (r *ScoreValidationRepository) LastCalibrationAt() (*time.Time, error) {
	var w models.ScoreCalibrationWeight
	err := r.db.Order("created_at DESC").First(&w).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &w.CreatedAt, nil
}
