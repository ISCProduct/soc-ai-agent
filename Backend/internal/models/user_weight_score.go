package models

import "time"

// UserWeightScore ユーザーごとの重みカテゴリ別スコア
type UserWeightScore struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	OrganizationID uint      `gorm:"not null;index;column:organization_id" json:"organization_id"`
	UserID         uint      `gorm:"not null;index:idx_user_category" json:"user_id"`
	SessionID      string    `gorm:"size:100;not null;index" json:"session_id"`
	WeightCategory string    `gorm:"size:100;not null;index:idx_user_category" json:"category"`
	Score          int       `gorm:"default:0" json:"score"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
