package models

import "time"

// ChatMessage チャット履歴を保存
type ChatMessage struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	OrganizationID   uint      `gorm:"not null;index;column:organization_id" json:"organization_id"`
	SessionID        string    `gorm:"size:100;not null;index" json:"session_id"`
	UserID           uint      `gorm:"not null;index" json:"user_id"`
	Role             string    `gorm:"size:20;not null" json:"role"` // "user" or "assistant"
	Content          string    `gorm:"type:text;not null" json:"content"`
	QuestionWeightID uint      `gorm:"index" json:"question_weight_id,omitempty"` // 質問に対応するQuestionWeightのID
	CreatedAt        time.Time `json:"created_at"`
}

// ChatSession チャットセッション情報
type ChatSession struct {
	SessionID     string    `json:"session_id"`
	UserID        uint      `json:"user_id"`
	StartedAt     time.Time `json:"started_at"`
	LastMessageAt time.Time `json:"last_message_at"`
	MessageCount  int       `json:"message_count"`
}
