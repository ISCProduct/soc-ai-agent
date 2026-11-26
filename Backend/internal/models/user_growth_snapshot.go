package models

import "time"

// UserGrowthSnapshot は、スコアか診断フェーズが進んだ時点の写し（SOCAIAGENT-601）。
// 1行が1回の記録。上書きしない。再計算の結果は入れない。
type UserGrowthSnapshot struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	UserID            uint      `gorm:"not null;index:idx_user_growth_snapshots_user_time" json:"user_id"`
	Source            string    `gorm:"size:32;not null" json:"source"`
	SessionID         string    `gorm:"size:100;not null" json:"session_id"`
	ScoresJSON        string    `gorm:"type:json;not null" json:"scores_json"`
	PhaseProgressJSON string    `gorm:"type:json;not null" json:"phase_progress_json"`
	CreatedAt         time.Time `gorm:"type:datetime(3);not null;autoCreateTime:milli" json:"created_at"`
}

func (UserGrowthSnapshot) TableName() string {
	return "user_growth_snapshots"
}
