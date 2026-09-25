package repositories

import (
	"Backend/internal/models"
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type APICallLogRepository struct {
	db *gorm.DB
}

func NewAPICallLogRepository(db *gorm.DB) *APICallLogRepository {
	return &APICallLogRepository{db: db}
}

func (r *APICallLogRepository) Create(log *models.APICallLog) error {
	return r.db.Create(log).Error
}

type DailyCostRow struct {
	Date         string
	TotalCostUSD float64
	TotalTokens  int64
	CallCount    int64
}

type MonthlyCostRow struct {
	Month        string
	TotalCostUSD float64
	TotalTokens  int64
	CallCount    int64
}

type ModelCostRow struct {
	Model        string
	TotalCostUSD float64
	TotalTokens  int64
	CallCount    int64
}

// DailyCosts は過去 nDays 日間の日次集計を返す
func (r *APICallLogRepository) DailyCosts(nDays int) ([]DailyCostRow, error) {
	var rows []DailyCostRow
	since := time.Now().UTC().AddDate(0, 0, -nDays)
	err := r.db.Raw(`
		SELECT DATE(called_at) AS date,
		       SUM(cost_usd) AS total_cost_usd,
		       SUM(total_tokens) AS total_tokens,
		       COUNT(*) AS call_count
		FROM api_call_logs
		WHERE called_at >= ?
		GROUP BY DATE(called_at)
		ORDER BY date ASC`, since).Scan(&rows).Error
	return rows, err
}

// MonthlyCosts は過去 nMonths ヶ月間の月次集計を返す
func (r *APICallLogRepository) MonthlyCosts(nMonths int) ([]MonthlyCostRow, error) {
	var rows []MonthlyCostRow
	since := time.Now().UTC().AddDate(0, -nMonths, 0)
	err := r.db.Raw(`
		SELECT DATE_FORMAT(called_at, '%Y-%m') AS month,
		       SUM(cost_usd) AS total_cost_usd,
		       SUM(total_tokens) AS total_tokens,
		       COUNT(*) AS call_count
		FROM api_call_logs
		WHERE called_at >= ?
		GROUP BY month
		ORDER BY month ASC`, since).Scan(&rows).Error
	return rows, err
}

// BreakdownDimension は利用量の集計軸（#1294）。
//
// 列名を SQL へ直接埋めるため、外から来る文字列をそのまま使わず許可リストで固定する。
type BreakdownDimension string

const (
	BreakdownByFeature      BreakdownDimension = "feature"
	BreakdownByProvider     BreakdownDimension = "provider"
	BreakdownByModel        BreakdownDimension = "model"
	BreakdownByOrganization BreakdownDimension = "organization"
)

// breakdownColumns は集計軸と実際の列名の対応。ここに無い軸は受け付けない。
var breakdownColumns = map[BreakdownDimension]string{
	BreakdownByFeature:      "feature",
	BreakdownByProvider:     "provider",
	BreakdownByModel:        "model",
	BreakdownByOrganization: "organization_id",
}

// UsageBreakdownRow は軸ごとの集計結果。
type UsageBreakdownRow struct {
	Key           string  `json:"key"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
	TotalTokens   int64   `json:"total_tokens"`
	CallCount     int64   `json:"call_count"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	CacheHitCount int64   `json:"cache_hit_count"`
}

// IsValidBreakdownDimension は集計軸が許可リストにあるかを返す。
func IsValidBreakdownDimension(dim BreakdownDimension) bool {
	_, ok := breakdownColumns[dim]
	return ok
}

// UsageBreakdown は指定軸で利用量を集計する（#1294）。
//
// 未設定の値（organization_id の NULL、provider の空文字）は 'unassigned' へ寄せる。
// 落とすと「合計は合うのに内訳が合わない」表になり、配賦漏れに気づけない。
func (r *APICallLogRepository) UsageBreakdown(ctx context.Context, since time.Time, dim BreakdownDimension) ([]UsageBreakdownRow, error) {
	col, ok := breakdownColumns[dim]
	if !ok {
		return nil, fmt.Errorf("unsupported breakdown dimension: %q", dim)
	}

	query := fmt.Sprintf(`
		SELECT COALESCE(NULLIF(CAST(%s AS CHAR), ''), 'unassigned') AS `+"`key`"+`,
		       SUM(cost_usd) AS total_cost_usd,
		       SUM(total_tokens) AS total_tokens,
		       COUNT(*) AS call_count,
		       AVG(latency_ms) AS avg_latency_ms,
		       SUM(cache_hit) AS cache_hit_count
		FROM api_call_logs
		WHERE called_at >= ?
		GROUP BY %s
		ORDER BY total_cost_usd DESC`, col, col)

	var rows []UsageBreakdownRow
	err := r.db.WithContext(ctx).Raw(query, since).Scan(&rows).Error
	return rows, err
}

// ModelBreakdown はモデル別コスト内訳を返す
func (r *APICallLogRepository) ModelBreakdown(since time.Time) ([]ModelCostRow, error) {
	var rows []ModelCostRow
	err := r.db.Raw(`
		SELECT model,
		       SUM(cost_usd) AS total_cost_usd,
		       SUM(total_tokens) AS total_tokens,
		       COUNT(*) AS call_count
		FROM api_call_logs
		WHERE called_at >= ?
		GROUP BY model
		ORDER BY total_cost_usd DESC`, since).Scan(&rows).Error
	return rows, err
}

// TotalFallbackCostSince は指定日時以降の「OpenAIフォールバック分だけ」の合計コストを返す（#1293）。
//
// フォールバックの USD 上限はこの値で判定する。全コール合計で判定すると、
// 企業検索など通常の OpenAI 利用だけで上限に達し、ローカル障害時に
// フォールバックが一度も発動しなくなる（実データで月 $57 に対し既定上限 $20）。
func (r *APICallLogRepository) TotalFallbackCostSince(ctx context.Context, since time.Time) (float64, error) {
	var total float64
	err := r.db.WithContext(ctx).Model(&models.APICallLog{}).
		Where("via_fallback = ? AND called_at >= ?", true, since).
		Select("COALESCE(SUM(cost_usd), 0)").
		Scan(&total).Error
	return total, err
}

// TotalCostSince は指定日時以降の合計コストを返す
func (r *APICallLogRepository) TotalCostSince(since time.Time) (float64, error) {
	var total float64
	err := r.db.Model(&models.APICallLog{}).
		Where("called_at >= ?", since).
		Select("COALESCE(SUM(cost_usd), 0)").
		Scan(&total).Error
	return total, err
}

// CountSearchCallsSince は指定日時以降の Web Search 系モデル呼び出し回数を返す。
// company Search 予算（#587）のカウントに使う。model に "search" を含む行を対象とする。
func (r *APICallLogRepository) CountSearchCallsSince(since time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.APICallLog{}).
		Where("called_at >= ? AND LOWER(model) LIKE ?", since, "%search%").
		Count(&count).Error
	return count, err
}
