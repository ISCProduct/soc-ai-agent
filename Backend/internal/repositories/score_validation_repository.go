package repositories

import (
	"Backend/internal/models"
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

// ── 相関分析クエリ ────────────────────────────────────────────────────────────

// CategoryCorrelationRow カテゴリ別スコア vs 通過率の相関行データ
type CategoryCorrelationRow struct {
	Category    string
	ScoreBand   string // "0-20", "21-40", "41-60", "61-80", "81-100"
	TotalCount  int
	PassCount   int    // document_passed / interview / offered / accepted のいずれか
	PassRate    float64
	AvgScore    float64
}

// GetCategoryPassRateCorrelation カテゴリ別スコア帯と選考通過率の相関を集計する
// 通過判定: status IN ('document_passed','interview','offered','accepted')
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
	err := r.db.Raw(`
		SELECT
			uws.weight_category AS category,
			FLOOR(uws.score / 20) AS score_band,
			COUNT(DISTINCT uas.id) AS total_count,
			SUM(CASE WHEN uas.status IN ('document_passed','interview','offered','accepted') THEN 1 ELSE 0 END) AS pass_count,
			AVG(uws.score) AS avg_score
		FROM user_weight_scores uws
		INNER JOIN user_application_statuses uas ON uas.user_id = uws.user_id
		WHERE uas.status != ''
		GROUP BY uws.weight_category, FLOOR(uws.score / 20)
		ORDER BY uws.weight_category, score_band
	`).Scan(&rows).Error
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
	PhaseName      string
	SessionCount   int
	AvgCompletion  float64
	PassCount      int
	PassRate       float64
}

// GetPhasePrecisionMetrics フェーズ別の完了率・通過率相関を集計する
func (r *ScoreValidationRepository) GetPhasePrecisionMetrics() ([]PhasePrecisionRow, error) {
	rows := []PhasePrecisionRow{}
	err := r.db.Raw(`
		SELECT
			ap.phase_name,
			COUNT(DISTINCT uap.session_id) AS session_count,
			AVG(uap.completion_score) AS avg_completion,
			COUNT(DISTINCT CASE WHEN uas.status IN ('document_passed','interview','offered','accepted') THEN uas.user_id END) AS pass_count,
			CASE
				WHEN COUNT(DISTINCT uap.user_id) = 0 THEN 0
				ELSE CAST(COUNT(DISTINCT CASE WHEN uas.status IN ('document_passed','interview','offered','accepted') THEN uas.user_id END) AS FLOAT) /
					COUNT(DISTINCT uap.user_id) * 100
			END AS pass_rate
		FROM user_analysis_progress uap
		INNER JOIN analysis_phases ap ON ap.id = uap.phase_id
		LEFT JOIN user_application_statuses uas ON uas.user_id = uap.user_id
		GROUP BY ap.phase_name, ap.phase_order
		ORDER BY ap.phase_order
	`).Scan(&rows).Error
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
	ExperimentName  string
	VariantName     string
	SessionCount    int
	PassCount       int
	PassRate        float64
	AvgScoreSum     float64
}

// GetVariantResults 実験バリアント別の通過率を集計する
func (r *ScoreValidationRepository) GetVariantResults(experimentName string) ([]VariantResultRow, error) {
	rows := []VariantResultRow{}
	err := r.db.Raw(`
		SELECT
			va.experiment_name,
			va.assigned_variant AS variant_name,
			COUNT(DISTINCT va.session_id) AS session_count,
			COUNT(DISTINCT CASE WHEN uas.status IN ('document_passed','interview','offered','accepted') THEN uas.user_id END) AS pass_count,
			CASE
				WHEN COUNT(DISTINCT va.user_id) = 0 THEN 0
				ELSE CAST(COUNT(DISTINCT CASE WHEN uas.status IN ('document_passed','interview','offered','accepted') THEN uas.user_id END) AS FLOAT) /
					COUNT(DISTINCT va.user_id) * 100
			END AS pass_rate
		FROM variant_assignments va
		LEFT JOIN user_application_statuses uas ON uas.user_id = va.user_id
		WHERE va.experiment_name = ?
		GROUP BY va.experiment_name, va.assigned_variant
	`, experimentName).Scan(&rows).Error
	return rows, err
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

// CategoryPassStats キャリブレーション計算用の生データ
type CategoryPassStats struct {
	Category  string
	AvgScore  float64
	PassRate  float64
	SampleN   int
}

func (r *ScoreValidationRepository) GetCategoryPassStats() ([]CategoryPassStats, error) {
	rows := []CategoryPassStats{}
	err := r.db.Raw(`
		SELECT
			uws.weight_category AS category,
			AVG(uws.score) AS avg_score,
			CASE
				WHEN COUNT(DISTINCT uas.id) = 0 THEN 0
				ELSE CAST(SUM(CASE WHEN uas.status IN ('document_passed','interview','offered','accepted') THEN 1 ELSE 0 END) AS FLOAT) /
					COUNT(DISTINCT uas.id) * 100
			END AS pass_rate,
			COUNT(DISTINCT uas.id) AS sample_n
		FROM user_weight_scores uws
		INNER JOIN user_application_statuses uas ON uas.user_id = uws.user_id
		GROUP BY uws.weight_category
		HAVING COUNT(DISTINCT uas.id) >= 5
	`).Scan(&rows).Error
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

// LastCalibrationAt 最後にキャリブレーションが実行された日時
func (r *ScoreValidationRepository) LastCalibrationAt() (*time.Time, error) {
	var w models.ScoreCalibrationWeight
	err := r.db.Order("created_at DESC").First(&w).Error
	if err != nil {
		return nil, err
	}
	return &w.CreatedAt, nil
}
