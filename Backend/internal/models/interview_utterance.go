package models

import "time"

// InterviewUtterance 面接中の発話ログ
type InterviewUtterance struct {
	ID        uint   `gorm:"primaryKey"`
	SessionID uint   `gorm:"index;not null;uniqueIndex:uniq_interview_utterances_session_client,priority:1"`
	Role      string `gorm:"size:16;index;not null"` // user / ai
	Text      string `gorm:"type:text;not null"`
	// ClientUtteranceID は発話ごとにクライアントが1つだけ発行するID(#1476)。
	// (session_id, client_utterance_id) が一意なので、保存の再試行で同じ発話が二重に入らない。
	// 空文字はNULLとして保存する（NULL同士は重複扱いにならず、ID未送信の経路は従来どおり追記）。
	ClientUtteranceID *string   `gorm:"size:64;uniqueIndex:uniq_interview_utterances_session_client,priority:2"`
	CreatedAt         time.Time `gorm:"index"`
}
