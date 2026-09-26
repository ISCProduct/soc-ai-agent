package models

import "time"

const (
	GuidanceKindLowMatch = "low_match"
	GuidanceKindResume   = "resume"
	GuidanceKindInactive = "inactive"
)

// TeacherStudentGuidance は教員から生徒への軌道修正・提案メッセージ（#1028 拡張）。
type TeacherStudentGuidance struct {
	ID                   uint       `gorm:"primaryKey" json:"id"`
	StudentUserID        uint       `gorm:"not null;index:idx_guidances_student_active" json:"student_user_id"`
	TeacherUserID        uint       `gorm:"not null;index" json:"teacher_user_id"`
	Kind                 string     `gorm:"size:32;not null" json:"kind"`
	Message              string     `gorm:"type:text;not null" json:"message"`
	SuggestedIndustries  string     `gorm:"type:json" json:"suggested_industries,omitempty"`
	DismissedAt          *time.Time `json:"dismissed_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

func (TeacherStudentGuidance) TableName() string {
	return "teacher_student_guidances"
}
