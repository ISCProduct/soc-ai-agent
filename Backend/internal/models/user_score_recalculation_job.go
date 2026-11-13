package models

import "time"

const (
	UserScoreRecalculationJobRunning   = "running"
	UserScoreRecalculationJobPaused    = "paused"
	UserScoreRecalculationJobCompleted = "completed"
	UserScoreRecalculationJobCancelled = "cancelled"
)

// 再計算に失敗したセッションの分類。last_error にはこの分類だけを保存し、
// エラー本文はサーバーログに出す（SQL や回答内容を管理画面へ出さないため）。
const (
	UserScoreRecalculationFailureLoad          = "load_failed"
	UserScoreRecalculationFailureNoScores      = "no_scores"
	UserScoreRecalculationFailureSave          = "save_failed"
	UserScoreRecalculationFailureTargetChanged = "target_changed"
	UserScoreRecalculationFailureMatching      = "matching_failed"
	// LLM が使えず、評価が欠けた値になるので保存しなかった。計算版も進めないので次のジョブで拾い直す（#1770 B3）
	UserScoreRecalculationFailureLLMDegraded = "llm_degraded"
	// 保存軸もテンプレートの軸も無い回答があり、再計算しなかった（スコアは残し、計算版だけ進める）（#1770 B2）
	UserScoreRecalculationFailureAxisUnknown = "axis_unknown"
)

type UserScoreRecalculationJob struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	CalculationVersion string     `gorm:"size:32;not null" json:"calculation_version"`
	Status             string     `gorm:"size:20;not null;index" json:"status"`
	MaxScoreID         uint       `gorm:"not null" json:"-"`
	CursorScoreID      uint       `gorm:"not null;default:0" json:"-"`
	TotalSessions      uint       `gorm:"not null;default:0" json:"total_sessions"`
	ProcessedSessions  uint       `gorm:"not null;default:0" json:"processed_sessions"`
	FailedSessions     uint       `gorm:"not null;default:0" json:"failed_sessions"`
	EstimatedCostUSD   float64    `gorm:"type:decimal(12,6);not null;default:0" json:"estimated_cost_usd"`
	LastError          string     `gorm:"type:text" json:"last_error,omitempty"`
	BatchLeaseToken    *string    `gorm:"type:char(36)" json:"-"`
	BatchLeaseUntil    *time.Time `gorm:"type:datetime(3)" json:"-"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (UserScoreRecalculationJob) TableName() string { return "user_score_recalculation_jobs" }

// UserScoreRecalculationSnapshot は再計算で置き換える前のチャットスコア。
// ScoresJSON は []UserWeightScore の JSON で、列が増えても失わないよう行全体を残す。
type UserScoreRecalculationSnapshot struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	JobID      uint      `gorm:"not null" json:"job_id"`
	UserID     uint      `gorm:"not null" json:"user_id"`
	SessionID  string    `gorm:"size:100;not null" json:"session_id"`
	ScoresJSON string    `gorm:"column:scores_json;type:json;not null" json:"-"`
	CreatedAt  time.Time `json:"created_at"`
}

func (UserScoreRecalculationSnapshot) TableName() string {
	return "user_score_recalculation_snapshots"
}
