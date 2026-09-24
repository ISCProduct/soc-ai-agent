package models

import "time"

// SchoolJobSuppression は学校ごとに個別停止された求人（#1508）。
//
// 企業の掲載承認（school_company_approvals）は企業単位だが、問題のある
// 求人だけを特定の学校向けに止めたい場合に使う。ここに行があると、
// その学校の学生一覧からその求人が除外される。
//
// スキーマは migrations/000038_school_job_suppressions で管理。
type SchoolJobSuppression struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	SchoolID      uint      `gorm:"not null;uniqueIndex:idx_sjs_school_job" json:"school_id"`
	JobPositionID uint      `gorm:"not null;uniqueIndex:idx_sjs_school_job" json:"job_position_id"`
	SuppressedBy  uint      `gorm:"not null" json:"suppressed_by"`
	Reason        string    `gorm:"type:text" json:"reason,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

func (SchoolJobSuppression) TableName() string {
	return "school_job_suppressions"
}
