package models

import "time"

// CollectiveInsightLog ユーザー行動の匿名集計ログ
// userIDはハッシュ化して保存し、個人を特定できないようにする
type CollectiveInsightLog struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	AnonymousUserID string `gorm:"type:varchar(64);not null;index" json:"anonymous_user_id"` // SHA-256ハッシュ
	CompanyID       uint   `gorm:"not null;index" json:"company_id"`
	ActionType      string `gorm:"type:varchar(50);not null" json:"action_type"` // viewed / applied / passed / rejected
	// スコアプロファイルのスナップショット（集合知計算用）
	ScoreSnapshot string    `gorm:"type:text" json:"score_snapshot"` // JSON: {"技術志向":80,"チームワーク":60,...}
	CreatedAt     time.Time `json:"created_at"`
}

// AnonymizedBehaviorSummary 企業別の匿名行動集計サマリー（定期バッチで更新）
type AnonymizedBehaviorSummary struct {
	ID         uint    `gorm:"primaryKey" json:"id"`
	CompanyID  uint    `gorm:"not null;uniqueIndex" json:"company_id"`
	ViewCount  int     `gorm:"default:0" json:"view_count"`
	ApplyCount int     `gorm:"default:0" json:"apply_count"`
	PassCount  int     `gorm:"default:0" json:"pass_count"`
	PassRate   float64 `gorm:"default:0" json:"pass_rate"`
	// 通過ユーザーのスコア平均（JSON）
	AvgPasserScores string    `gorm:"type:text" json:"avg_passer_scores"`
	UpdatedAt       time.Time `json:"updated_at"`
}
