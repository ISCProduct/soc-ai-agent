package repositories

import (
	"Backend/internal/models"
	"context"
	"errors"
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

func (r *AIBudgetRepository) ListSettings(ctx context.Context) ([]models.AIBudgetSetting, error) {
	var settings []models.AIBudgetSetting
	err := r.db.WithContext(ctx).Order("scope ASC, scope_id ASC").Find(&settings).Error
	return settings, err
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

// UsageCostSince は利用者または学校に配賦されたAPI・リアルタイム音声コストを返す。
func (r *AIBudgetRepository) UsageCostSince(ctx context.Context, scope string, scopeID uint, since, until time.Time) (float64, error) {
	var apiCost, realtimeCost float64
	apiQuery := r.db.WithContext(ctx).Model(&models.APICallLog{}).
		Where("api_call_logs.called_at >= ?", since)
	realtimeQuery := r.db.WithContext(ctx).Model(&models.RealtimeUsageLog{}).
		Where("realtime_usage_logs.started_at >= ?", since)
	if !until.IsZero() {
		apiQuery = apiQuery.Where("api_call_logs.called_at < ?", until)
		realtimeQuery = realtimeQuery.Where("realtime_usage_logs.started_at < ?", until)
	}
	switch scope {
	case models.AIBudgetScopeUser:
		apiQuery = apiQuery.Where("api_call_logs.user_id = ?", scopeID)
		realtimeQuery = realtimeQuery.Where("realtime_usage_logs.user_id = ?", scopeID)
	case models.AIBudgetScopeSchool:
		apiQuery = apiQuery.Joins("JOIN users ON users.id = api_call_logs.user_id").
			Where("users.school_id = ?", scopeID)
		realtimeQuery = realtimeQuery.Joins("JOIN users ON users.id = realtime_usage_logs.user_id").
			Where("users.school_id = ?", scopeID)
	default:
		return 0, gorm.ErrInvalidData
	}
	if err := apiQuery.Select("COALESCE(SUM(api_call_logs.cost_usd), 0)").Scan(&apiCost).Error; err != nil {
		return 0, err
	}
	if err := realtimeQuery.Select("COALESCE(SUM(realtime_usage_logs.cost_usd), 0)").Scan(&realtimeCost).Error; err != nil {
		return 0, err
	}
	return apiCost + realtimeCost, nil
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
