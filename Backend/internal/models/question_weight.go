package models

import "time"

// QuestionWeight 質問と重み係数を管理するテーブル
type QuestionWeight struct {
	ID             uint      `gorm:"primaryKey" json:"ID"`
	Question       string    `gorm:"type:varchar(500);not null;uniqueIndex:idx_question_hash" json:"question"`
	WeightCategory string    `gorm:"size:100;not null;index" json:"weight_category"` // 例: "技術志向", "コミュニケーション力", "リーダーシップ志向"（正典: domain/valueobject/match.go）
	WeightValue    int       `gorm:"not null" json:"weight_value"`                   // 重み係数値
	IndustryID     uint      `gorm:"index" json:"industry_id,omitempty"`             // 関連する業界ID(オプション)
	JobCategoryID  uint      `gorm:"index" json:"job_category_id,omitempty"`         // 関連する職種ID(オプション)
	Description    string    `gorm:"type:text" json:"description,omitempty"`         // 質問の意図・説明
	IsActive       bool      `gorm:"default:true;index" json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
