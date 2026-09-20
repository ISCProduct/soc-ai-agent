package models

import "time"

// ChatMessage チャット履歴を保存
type ChatMessage struct {
	ID               uint   `gorm:"primaryKey" json:"id"`
	OrganizationID   uint   `gorm:"not null;index;column:organization_id" json:"organization_id"`
	SessionID        string `gorm:"size:100;not null;index" json:"session_id"`
	UserID           uint   `gorm:"not null;index" json:"user_id"`
	Role             string `gorm:"size:20;not null" json:"role"` // "user" or "assistant"
	Content          string `gorm:"type:text;not null" json:"content"`
	QuestionWeightID uint   `gorm:"index" json:"question_weight_id,omitempty"` // 質問に対応するQuestionWeightのID
	// WeightCategory は出題時に狙った評価軸(正典: domain/valueobject/match.go)。
	// 採点側が質問文から推測し直すと、キーワードに当たらない質問が既定の
	// 技術志向に落ちて狙った軸が永久に未評価のまま残る(#1333)。assistant の
	// 質問行にだけ入る。空なら従来どおり推測にフォールバックする。
	WeightCategory string    `gorm:"size:100" json:"weight_category,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// ChatSession チャットセッション情報
type ChatSession struct {
	SessionID     string    `json:"session_id"`
	UserID        uint      `json:"user_id"`
	StartedAt     time.Time `json:"started_at"`
	LastMessageAt time.Time `json:"last_message_at"`
	MessageCount  int       `json:"message_count"`
}
