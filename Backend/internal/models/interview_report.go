package models

import "time"

// InterviewReport 面接後の要約・評価
type InterviewReport struct {
	SessionID         uint   `gorm:"primaryKey"                    json:"session_id"`
	SummaryText       string `gorm:"type:text"                     json:"summary_text"`
	ScoresJSON        string `gorm:"type:json"                     json:"scores_json"`
	EvidenceJSON      string `gorm:"type:json"                     json:"evidence_json"`
	StrengthsJSON     string `gorm:"type:json"                     json:"strengths_json"`
	ImprovementsJSON  string `gorm:"type:json"                     json:"improvements_json"`
	TeacherReportJSON string `gorm:"type:json"                     json:"teacher_report_json"` // 教員用詳細レポート
	// ScoresAppliedAt は面接スコアを user_weight_scores へ反映した時刻（#1512）。
	// NULL なら未反映。反映する側が NULL から条件付き UPDATE で奪い合い、
	// 勝った1つだけがスコアを書く。Redis 障害中に複数タスクで同じセッションの
	// レポート生成が走っても、移動平均へ二重に反映されない。
	ScoresAppliedAt *time.Time `gorm:"index"                        json:"scores_applied_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
