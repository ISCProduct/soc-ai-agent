package models

import "time"

const (
	AIBudgetScopeUser   = "user"
	AIBudgetScopeSchool = "school"
)

// AIBudgetSetting は利用者または学校に設定されたAI利用上限を表す。
// 一意制約は migrations/000065 で定義する。AutoMigrate は使わない方針のためタグには書かない。
type AIBudgetSetting struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Scope           string    `gorm:"size:16;not null" json:"scope"`
	ScopeID         uint      `gorm:"not null" json:"scope_id"`
	DailyLimitUSD   float64   `gorm:"type:decimal(12,6);not null;default:0" json:"daily_limit_usd"`
	MonthlyLimitUSD float64   `gorm:"type:decimal(12,6);not null;default:0" json:"monthly_limit_usd"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (AIBudgetSetting) TableName() string {
	return "ai_budget_settings"
}
