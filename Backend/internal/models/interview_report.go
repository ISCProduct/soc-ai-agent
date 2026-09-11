package models

import "time"

// InterviewReport 面接後の要約・評価
type InterviewReport struct {
	SessionID         uint      `gorm:"primaryKey"                    json:"session_id"`
	SummaryText       string    `gorm:"type:text"                     json:"summary_text"`
	ScoresJSON        string    `gorm:"type:json"                     json:"scores_json"`
	EvidenceJSON      string    `gorm:"type:json"                     json:"evidence_json"`
	StrengthsJSON     string    `gorm:"type:json"                     json:"strengths_json"`
	ImprovementsJSON  string    `gorm:"type:json"                     json:"improvements_json"`
	TeacherReportJSON string    `gorm:"type:json"                     json:"teacher_report_json"` // 教員用詳細レポート
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
