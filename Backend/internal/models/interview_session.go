package models

import "time"

// InterviewSession 面接セッション
type InterviewSession struct {
	ID                uint       `gorm:"primaryKey"`
	UserID            uint       `gorm:"index;not null"`
	Status            string     `gorm:"size:32;index;not null"`
	Language          string     `gorm:"size:16;not null;default:'ja'"`
	InterviewerGender string     `gorm:"size:16;not null;default:'female'"`
	StartedAt         *time.Time `gorm:"index"`
	EndedAt           *time.Time `gorm:"index"`
	EstimatedCostUSD  float64    `gorm:"default:0"`
	TemplateVersion   string     `gorm:"size:32;default:'v1'"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time `gorm:"index"`
}
