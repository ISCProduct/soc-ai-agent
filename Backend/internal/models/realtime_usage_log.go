package models

import "time"

// RealtimeUsageLog records usage for OpenAI Realtime interview sessions.
type RealtimeUsageLog struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	UserID             uint       `gorm:"index;not null" json:"user_id"`
	InterviewSessionID uint       `gorm:"index;not null" json:"interview_session_id"`
	Status             string     `gorm:"size:16;index;not null;default:'active'" json:"status"`
	StartedAt          time.Time  `gorm:"not null;index" json:"started_at"`
	EndedAt            *time.Time `gorm:"index" json:"ended_at,omitempty"`
	DurationSeconds    int        `gorm:"not null;default:0" json:"duration_seconds"`
	CostUSD            float64    `gorm:"default:0" json:"cost_usd"`
	// トークンベースコスト計算用（response.done イベントから取得）
	InputTextTokens        int       `gorm:"not null;default:0" json:"input_text_tokens"`
	OutputTextTokens       int       `gorm:"not null;default:0" json:"output_text_tokens"`
	InputAudioTokens       int       `gorm:"not null;default:0" json:"input_audio_tokens"`
	OutputAudioTokens      int       `gorm:"not null;default:0" json:"output_audio_tokens"`
	InputCachedAudioTokens int       `gorm:"not null;default:0" json:"input_cached_audio_tokens"`
	TokenBasedCost         bool      `gorm:"not null;default:false" json:"token_based_cost"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}
