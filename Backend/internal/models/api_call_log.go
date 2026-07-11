package models

import "time"

// APICallLog OpenAI APIコール記録
type APICallLog struct {
	ID               uint      `gorm:"primaryKey"           json:"id"`
	Model            string    `gorm:"size:100;not null"    json:"model"`
	PromptTokens     int       `gorm:"not null;default:0"   json:"prompt_tokens"`
	CompletionTokens int       `gorm:"not null;default:0"   json:"completion_tokens"`
	TotalTokens      int       `gorm:"not null;default:0"   json:"total_tokens"`
	CostUSD          float64   `gorm:"default:0" json:"cost_usd"`
	CalledAt         time.Time `gorm:"not null;index"       json:"called_at"`
}
