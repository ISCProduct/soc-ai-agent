package repositories

import (
	"Backend/internal/models"
	"time"

	"gorm.io/gorm"
)

type RealtimeUsageRepository struct {
	db *gorm.DB
}

func NewRealtimeUsageRepository(db *gorm.DB) *RealtimeUsageRepository {
	return &RealtimeUsageRepository{db: db}
}

func (r *RealtimeUsageRepository) FindActiveBySessionID(sessionID uint) (*models.RealtimeUsageLog, error) {
	var log models.RealtimeUsageLog
	err := r.db.Where("interview_session_id = ? AND status = ? AND ended_at IS NULL", sessionID, "active").
		Order("started_at DESC").
		First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *RealtimeUsageRepository) Create(log *models.RealtimeUsageLog) error {
	return r.db.Create(log).Error
}

func (r *RealtimeUsageRepository) Update(log *models.RealtimeUsageLog) error {
	return r.db.Save(log).Error
}

func (r *RealtimeUsageRepository) CountActiveSessions() (int64, error) {
	var count int64
	err := r.db.Model(&models.RealtimeUsageLog{}).
		Where("status = ? AND ended_at IS NULL", "active").
		Count(&count).Error
	return count, err
}

type RealtimeDailyRow struct {
	Date                   string
	TotalCostUSD           float64
	TotalDurationSeconds   int64
	SessionCount           int64
	UserCount              int64
	TotalInputAudioTokens  int64
	TotalOutputAudioTokens int64
	TotalInputTextTokens   int64
	TotalOutputTextTokens  int64
}

type RealtimeMonthlyRow struct {
	Month                  string
	TotalCostUSD           float64
	TotalDurationSeconds   int64
	SessionCount           int64
	UserCount              int64
	TotalInputAudioTokens  int64
	TotalOutputAudioTokens int64
	TotalInputTextTokens   int64
	TotalOutputTextTokens  int64
}

type RealtimeUserRow struct {
	UserID                 uint
	TotalCostUSD           float64
	TotalDurationSeconds   int64
	SessionCount           int64
	TotalInputAudioTokens  int64
	TotalOutputAudioTokens int64
	TotalInputTextTokens   int64
	TotalOutputTextTokens  int64
}

func (r *RealtimeUsageRepository) DailyUsage(nDays int, currentRatePerMinute float64) ([]RealtimeDailyRow, error) {
	var rows []RealtimeDailyRow
	since := time.Now().UTC().AddDate(0, 0, -nDays)
	err := r.db.Raw(`
		SELECT DATE(started_at) AS date,
		       SUM(CASE
		             WHEN ended_at IS NULL THEN TIMESTAMPDIFF(SECOND, started_at, UTC_TIMESTAMP()) * (? / 60.0)
		             ELSE cost_usd
		           END) AS total_cost_usd,
		       SUM(CASE
		             WHEN ended_at IS NULL THEN TIMESTAMPDIFF(SECOND, started_at, UTC_TIMESTAMP())
		             ELSE duration_seconds
		           END) AS total_duration_seconds,
		       COUNT(DISTINCT interview_session_id) AS session_count,
		       COUNT(DISTINCT user_id) AS user_count,
		       SUM(input_audio_tokens) AS total_input_audio_tokens,
		       SUM(output_audio_tokens) AS total_output_audio_tokens,
		       SUM(input_text_tokens) AS total_input_text_tokens,
		       SUM(output_text_tokens) AS total_output_text_tokens
		FROM realtime_usage_logs
		WHERE started_at >= ?
		GROUP BY DATE(started_at)
		ORDER BY date ASC
	`, currentRatePerMinute, since).Scan(&rows).Error
	return rows, err
}

func (r *RealtimeUsageRepository) MonthlyUsage(nMonths int, currentRatePerMinute float64) ([]RealtimeMonthlyRow, error) {
	var rows []RealtimeMonthlyRow
	since := time.Now().UTC().AddDate(0, -nMonths, 0)
	err := r.db.Raw(`
		SELECT DATE_FORMAT(started_at, '%Y-%m') AS month,
		       SUM(CASE
		             WHEN ended_at IS NULL THEN TIMESTAMPDIFF(SECOND, started_at, UTC_TIMESTAMP()) * (? / 60.0)
		             ELSE cost_usd
		           END) AS total_cost_usd,
		       SUM(CASE
		             WHEN ended_at IS NULL THEN TIMESTAMPDIFF(SECOND, started_at, UTC_TIMESTAMP())
		             ELSE duration_seconds
		           END) AS total_duration_seconds,
		       COUNT(DISTINCT interview_session_id) AS session_count,
		       COUNT(DISTINCT user_id) AS user_count,
		       SUM(input_audio_tokens) AS total_input_audio_tokens,
		       SUM(output_audio_tokens) AS total_output_audio_tokens,
		       SUM(input_text_tokens) AS total_input_text_tokens,
		       SUM(output_text_tokens) AS total_output_text_tokens
		FROM realtime_usage_logs
		WHERE started_at >= ?
		GROUP BY month
		ORDER BY month ASC
	`, currentRatePerMinute, since).Scan(&rows).Error
	return rows, err
}

func (r *RealtimeUsageRepository) UserBreakdown(since time.Time, currentRatePerMinute float64, limit int) ([]RealtimeUserRow, error) {
	var rows []RealtimeUserRow
	query := r.db.Raw(`
		SELECT user_id,
		       SUM(CASE
		             WHEN ended_at IS NULL THEN TIMESTAMPDIFF(SECOND, started_at, UTC_TIMESTAMP()) * (? / 60.0)
		             ELSE cost_usd
		           END) AS total_cost_usd,
		       SUM(CASE
		             WHEN ended_at IS NULL THEN TIMESTAMPDIFF(SECOND, started_at, UTC_TIMESTAMP())
		             ELSE duration_seconds
		           END) AS total_duration_seconds,
		       COUNT(DISTINCT interview_session_id) AS session_count,
		       SUM(input_audio_tokens) AS total_input_audio_tokens,
		       SUM(output_audio_tokens) AS total_output_audio_tokens,
		       SUM(input_text_tokens) AS total_input_text_tokens,
		       SUM(output_text_tokens) AS total_output_text_tokens
		FROM realtime_usage_logs
		WHERE started_at >= ?
		GROUP BY user_id
		ORDER BY total_cost_usd DESC
	`, currentRatePerMinute, since)

	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Scan(&rows).Error
	return rows, err
}

func (r *RealtimeUsageRepository) CurrentMonthTotalCostEstimated(currentRatePerMinute float64) (float64, error) {
	now := time.Now().UTC()
	since := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	var total float64
	err := r.db.Raw(`
		SELECT COALESCE(SUM(
			CASE
				WHEN ended_at IS NULL THEN TIMESTAMPDIFF(SECOND, started_at, UTC_TIMESTAMP()) * (? / 60.0)
				ELSE cost_usd
			END
		), 0) AS total
		FROM realtime_usage_logs
		WHERE started_at >= ?
	`, currentRatePerMinute, since).Scan(&total).Error
	return total, err
}
