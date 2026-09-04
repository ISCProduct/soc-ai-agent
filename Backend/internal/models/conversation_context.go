package models

import "time"

type ConversationContext struct {
	ID             uint   `gorm:"primaryKey"`
	UserID         uint   `gorm:"not null;index"`
	SessionID      string `gorm:"size:100;not null;index"`
	IndustryIDs    string `gorm:"type:json"`
	JobCategoryIDs string `gorm:"type:json"`
	AnswerHistory  string `gorm:"type:text"`
	CurrentPhase   string `gorm:"size:50"`
	TotalScore     int    `gorm:"default:0"`
	// LlmSummary はセッションに対する LLM 生成の診断サマリ（日本語の短い要約）を保存します。
	LlmSummary string `gorm:"type:text" json:"llm_summary,omitempty"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
