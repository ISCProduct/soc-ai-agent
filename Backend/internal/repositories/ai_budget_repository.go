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

// AIBudgetUsage は利用者または学校の日次・月次の利用額。
type AIBudgetUsage struct {
	Daily   float64
	Monthly float64
}

// UsageCosts は利用者または学校に配賦された日次・月次の利用額を返す。
func (r *AIBudgetRepository) UsageCosts(
	ctx context.Context,
	scope string,
	scopeID uint,
	dayStart, monthStart time.Time,
	realtimeRatePerMinute float64,
) (daily, monthly float64, err error) {
	usage, err := r.UsageCostsByScope(ctx, scope, []uint{scopeID}, dayStart, monthStart, realtimeRatePerMinute)
	if err != nil {
		return 0, 0, err
	}
	return usage[scopeID].Daily, usage[scopeID].Monthly, nil
}

// UsageCostsByScope は、同じ種類の対象（利用者または学校）の利用額を1本の集計で返す。
// 利用のない対象は結果に含まれない（ゼロ値で読める）。予算判定と管理画面の一覧で
// 集計の式を共有するため、1件の判定もこれを通す。
//
// 終了していないリアルタイム音声は、cost_usd が終了時に書かれるので、経過時間と
// realtimeRatePerMinute から見積もる。離脱したセッションを 0 として数えると上限を回避できる。
//   - 日次は、当日0時以降の経過だけを足す。日付をまたいで続く通話も新しい日の上限に入る。
//   - 月次は、開始から現在までを開始月に足す（月をまたいだ継続は開始月に入る）。
//   - started_at が現在より後なら 0 にする。負の見積もりが合計を減らし、上限をすり抜けないように。
//
// 現在時刻は DB の関数ではなく引数で渡す。started_at と同じ DSN の loc で変換されるので、
// コンテナの TZ を変えても経過時間がずれない。
func (r *AIBudgetRepository) UsageCostsByScope(
	ctx context.Context,
	scope string,
	scopeIDs []uint,
	dayStart, monthStart time.Time,
	realtimeRatePerMinute float64,
) (map[uint]AIBudgetUsage, error) {
	var column string
	switch scope {
	case models.AIBudgetScopeUser:
		column = "user_id"
	case models.AIBudgetScopeSchool:
		column = "school_id"
	default:
		return nil, gorm.ErrInvalidData
	}
	result := make(map[uint]AIBudgetUsage, len(scopeIDs))
	if len(scopeIDs) == 0 {
		return result, nil
	}
	now := time.Now().UTC()
	var rows []struct {
		ScopeID uint
		Daily   float64
		Monthly float64
	}
	if err := r.db.WithContext(ctx).Raw(fmt.Sprintf(`
SELECT scope_id, COALESCE(SUM(daily), 0) AS daily, COALESCE(SUM(monthly), 0) AS monthly
FROM (
  SELECT %[1]s AS scope_id,
    CASE WHEN called_at >= ? THEN cost_usd ELSE 0 END AS daily,
    cost_usd AS monthly
  FROM api_call_logs
  WHERE %[1]s IN ? AND called_at >= ?
  UNION ALL
  SELECT %[1]s AS scope_id,
    CASE
      WHEN ended_at IS NULL THEN
        GREATEST(TIMESTAMPDIFF(SECOND, GREATEST(started_at, CAST(? AS DATETIME(3))), ?), 0) * (? / 60.0)
      WHEN started_at >= ? THEN cost_usd
      ELSE 0
    END AS daily,
    CASE
      WHEN ended_at IS NULL THEN GREATEST(TIMESTAMPDIFF(SECOND, started_at, ?), 0) * (? / 60.0)
      ELSE cost_usd
    END AS monthly
  FROM realtime_usage_logs
  WHERE %[1]s IN ? AND started_at >= ?
) costs_by_scope
GROUP BY scope_id`, column),
		dayStart, scopeIDs, monthStart,
		dayStart, now, realtimeRatePerMinute, dayStart,
		now, realtimeRatePerMinute,
		scopeIDs, monthStart,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ScopeID] = AIBudgetUsage{Daily: row.Daily, Monthly: row.Monthly}
	}
	return result, nil
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
