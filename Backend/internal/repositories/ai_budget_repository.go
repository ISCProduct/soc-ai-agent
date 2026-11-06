package repositories

import (
	"Backend/internal/models"
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AIBudgetRepository は予算設定と利用実績を永続化する。
type AIBudgetRepository struct {
	db *gorm.DB
}

func NewAIBudgetRepository(db *gorm.DB) *AIBudgetRepository {
	return &AIBudgetRepository{db: db}
}

func (r *AIBudgetRepository) FindSetting(ctx context.Context, scope string, scopeID uint) (*models.AIBudgetSetting, error) {
	var setting models.AIBudgetSetting
	err := r.db.WithContext(ctx).Where("scope = ? AND scope_id = ?", scope, scopeID).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

func (r *AIBudgetRepository) ListSettings(ctx context.Context, limit, offset int) ([]models.AIBudgetSetting, int64, error) {
	var settings []models.AIBudgetSetting
	query := r.db.WithContext(ctx).Model(&models.AIBudgetSetting{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("scope ASC, scope_id ASC").Limit(limit).Offset(offset).Find(&settings).Error
	return settings, total, err
}

func (r *AIBudgetRepository) SaveSetting(ctx context.Context, setting *models.AIBudgetSetting) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "scope"}, {Name: "scope_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"daily_limit_usd", "monthly_limit_usd", "updated_at",
		}),
	}).Create(setting).Error
}

func (r *AIBudgetRepository) DeleteSetting(ctx context.Context, scope string, scopeID uint) error {
	return r.db.WithContext(ctx).Where("scope = ? AND scope_id = ?", scope, scopeID).
		Delete(&models.AIBudgetSetting{}).Error
}

func (r *AIBudgetRepository) SchoolIDForUser(ctx context.Context, userID uint) (*uint, error) {
	var row struct {
		SchoolID *uint
	}
	err := r.db.WithContext(ctx).Model(&models.User{}).Select("school_id").Where("id = ?", userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row.SchoolID, nil
}

// aiBudgetCosts は1つの表から読んだ日次・月次の利用額。
type aiBudgetCosts struct {
	Daily   float64
	Monthly float64
}

// UsageCosts は利用者または学校に配賦された日次・月次の利用額を返す。
//
// realtimeRatePerMinute は終了していないリアルタイム音声の見積もりに使う。cost_usd は終了時に
// 書かれるので、離脱したセッションを 0 として数えると上限を回避できてしまう。
// 管理画面のコスト集計（realtime_usage_repository.go）と同じ見積もり方に揃える。
func (r *AIBudgetRepository) UsageCosts(
	ctx context.Context,
	scope string,
	scopeID uint,
	dayStart, monthStart time.Time,
	realtimeRatePerMinute float64,
) (daily, monthly float64, err error) {
	var column string
	switch scope {
	case models.AIBudgetScopeUser:
		column = "user_id"
	case models.AIBudgetScopeSchool:
		column = "school_id"
	default:
		return 0, 0, gorm.ErrInvalidData
	}

	var api, realtime aiBudgetCosts
	if err := r.db.WithContext(ctx).Raw(fmt.Sprintf(`
SELECT
  COALESCE(SUM(CASE WHEN called_at >= ? THEN cost_usd ELSE 0 END), 0) AS daily,
  COALESCE(SUM(cost_usd), 0) AS monthly
FROM api_call_logs
WHERE %s = ? AND called_at >= ?`, column), dayStart, scopeID, monthStart).Scan(&api).Error; err != nil {
		return 0, 0, err
	}
	const realtimeCost = `CASE WHEN ended_at IS NULL THEN TIMESTAMPDIFF(SECOND, started_at, UTC_TIMESTAMP()) * (? / 60.0) ELSE cost_usd END`
	if err := r.db.WithContext(ctx).Raw(fmt.Sprintf(`
SELECT
  COALESCE(SUM(CASE WHEN started_at >= ? THEN %[1]s ELSE 0 END), 0) AS daily,
  COALESCE(SUM(%[1]s), 0) AS monthly
FROM realtime_usage_logs
WHERE %[2]s = ? AND started_at >= ?`, realtimeCost, column),
		dayStart, realtimeRatePerMinute, realtimeRatePerMinute, scopeID, monthStart).Scan(&realtime).Error; err != nil {
		return 0, 0, err
	}
	return api.Daily + realtime.Daily, api.Monthly + realtime.Monthly, nil
}

func (r *AIBudgetRepository) ScopeExists(ctx context.Context, scope string, scopeID uint) (bool, error) {
	var count int64
	var model any
	switch scope {
	case models.AIBudgetScopeUser:
		model = &models.User{}
	case models.AIBudgetScopeSchool:
		model = &models.School{}
	default:
		return false, gorm.ErrInvalidData
	}
	err := r.db.WithContext(ctx).Model(model).Where("id = ?", scopeID).Count(&count).Error
	return count > 0, err
}
