package models

import "time"

// InterviewShadowScore はローカル採点の結果。学生の画面やマッチングには出さない。
type InterviewShadowScore struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SessionID    uint      `gorm:"uniqueIndex:uniq_interview_shadow_scores_session;not null" json:"session_id"`
	ModelVersion string    `gorm:"size:64;not null" json:"model_version"`
	ScoresJSON   string    `gorm:"type:json;not null" json:"scores_json"`
	LatencyMs    int       `gorm:"not null" json:"latency_ms"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
