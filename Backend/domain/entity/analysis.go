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
