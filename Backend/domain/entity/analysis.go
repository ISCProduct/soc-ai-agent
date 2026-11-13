package entity

import "time"

// UserWeightScore ユーザーの重みカテゴリ別スコアエンティティ
type UserWeightScore struct {
	ID                 uint
	UserID             uint
	SessionID          string
	WeightCategory     string
	Score              int
	CalculationVersion string
	PromptVersion      string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// SessionValidation セッションバリデーション状態エンティティ
type SessionValidation struct {
	ID                    uint
	SessionID             string
	InvalidAnswerCount    int
	IsTerminated          bool
	LastInvalidAnswerTime *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
