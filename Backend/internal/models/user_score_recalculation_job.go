package models

import "time"

const (
	UserScoreRecalculationJobRunning   = "running"
	UserScoreRecalculationJobPaused    = "paused"
	UserScoreRecalculationJobCompleted = "completed"
)

type UserScoreRecalculationJob struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	CalculationVersion string    `gorm:"size:32;not null" json:"calculation_version"`
	Status             string    `gorm:"size:20;not null;index" json:"status"`
	MaxScoreID         uint      `gorm:"not null" json:"-"`
	CursorScoreID      uint      `gorm:"not null;default:0" json:"-"`
	TotalSessions      uint      `gorm:"not null;default:0" json:"total_sessions"`
	ProcessedSessions  uint      `gorm:"not null;default:0" json:"processed_sessions"`
	FailedSessions     uint      `gorm:"not null;default:0" json:"failed_sessions"`
	EstimatedCostUSD   float64   `gorm:"type:decimal(12,6);not null;default:0" json:"estimated_cost_usd"`
	LastError          string    `gorm:"type:text" json:"-"`
	BatchLeaseToken    *string   `gorm:"type:char(36)" json:"-"`
	BatchLeaseUntil    *time.Time `gorm:"type:datetime(3)" json:"-"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (UserScoreRecalculationJob) TableName() string { return "user_score_recalculation_jobs" }
