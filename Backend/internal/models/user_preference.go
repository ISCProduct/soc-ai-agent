package models

import "time"

// UserPreference は学生の希望条件（#1094）。
// 企業側の学生検索フィルタ軸（希望業界・希望勤務地・希望職種）を保持する。
// スキーマは migrations/000018_scout_student_search.up.sql で管理。
type UserPreference struct {
	ID                   uint      `gorm:"primaryKey" json:"id"`
	UserID               uint      `gorm:"not null;uniqueIndex" json:"user_id"`
	DesiredIndustryID    *uint     `gorm:"index" json:"desired_industry_id,omitempty"`
	DesiredJobCategoryID *uint     `json:"desired_job_category_id,omitempty"`
	DesiredLocation      string    `gorm:"type:varchar(100);not null;default:''" json:"desired_location"`
	Note                 string    `gorm:"type:text" json:"note"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func (UserPreference) TableName() string { return "user_preferences" }
