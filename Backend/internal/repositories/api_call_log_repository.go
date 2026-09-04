package repositories

import (
	"Backend/internal/models"
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
