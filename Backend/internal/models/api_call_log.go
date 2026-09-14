package models

import "time"

// APICallLog OpenAI APIコール記録
type APICallLog struct {
	ID               uint    `gorm:"primaryKey"           json:"id"`
	Model            string  `gorm:"size:100;not null"    json:"model"`
	PromptTokens     int     `gorm:"not null;default:0"   json:"prompt_tokens"`
	CompletionTokens int     `gorm:"not null;default:0"   json:"completion_tokens"`
	TotalTokens      int     `gorm:"not null;default:0"   json:"total_tokens"`
	CostUSD          float64 `gorm:"default:0" json:"cost_usd"`
	// Provider / ViaFallback は課金対象の切り分けに使う（#1293）。
	// api_call_logs は推論先に関係なく記録されるため、これが無いと
	// ローカル推論（無料）や通常の OpenAI 利用とフォールバック分を区別できない。
	Provider    string    `gorm:"size:32;not null;default:''" json:"provider"`
	ViaFallback bool      `gorm:"not null;default:0;index:idx_api_call_logs_fallback_called_at,priority:1" json:"via_fallback"`
	CalledAt    time.Time `gorm:"not null;index;index:idx_api_call_logs_fallback_called_at,priority:2" json:"called_at"`
}
