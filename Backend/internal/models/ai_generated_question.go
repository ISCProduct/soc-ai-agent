package models

import "time"

type AIGeneratedQuestion struct {
	ID            uint                `gorm:"primaryKey"`
	UserID        uint                `gorm:"not null;index"`
	SessionID     string              `gorm:"size:100;not null;index"`
	TemplateID    *uint               `gorm:"index"` // NULLを許容（AI生成の場合）
	Template      *AIQuestionTemplate `gorm:"foreignKey:TemplateID"`
	QuestionText  string              `gorm:"type:text;not null"`
	Weight        int                 `gorm:"not null"`
	AnswerText    string              `gorm:"type:text"`
	AnswerScore   int                 `gorm:"default:0"`
	IsAnswered    bool                `gorm:"default:false"`
	PromptVersion string              `gorm:"size:64;not null;default:''" json:"prompt_version,omitempty"`
	ContextData   string              `gorm:"type:json"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
