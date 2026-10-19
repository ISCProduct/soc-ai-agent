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
	// 索引は張らない。検索条件は `session_id = ? AND scores_applied_at IS NULL` で、
	// session_id が主キーなので1行に絞れている。scores_applied_at 単体の索引は
	// 使われない。
	//
	// `gorm:"index"` と書いても索引はできない。このプロジェクトは AutoMigrate を
	// 使わず、スキーマは Backend/migrations の SQL が唯一の定義（CLAUDE.md）。
	// タグだけ書くと「索引がある」と誤解させる。
	ScoresAppliedAt *time.Time `json:"scores_applied_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
