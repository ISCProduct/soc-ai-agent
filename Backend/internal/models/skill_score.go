package models

import "time"

// SkillCategory スキルカテゴリ
type SkillCategory string

const (
	SkillCategoryFrontend SkillCategory = "Frontend"
	SkillCategoryBackend  SkillCategory = "Backend"
	SkillCategoryInfra    SkillCategory = "Infrastructure"
	SkillCategoryDB       SkillCategory = "Database"
	SkillCategoryOther    SkillCategory = "Other"
)

// SkillScore ユーザーのカテゴリ別スキルスコア
type SkillScore struct {
	ID        uint          `gorm:"primaryKey"`
	UserID    uint          `gorm:"index;not null"`
	Category  SkillCategory `gorm:"size:50;not null"`
	Score     float64       // 0〜100 のスコア
	CreatedAt time.Time
	UpdatedAt time.Time
}
