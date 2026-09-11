package models

import "time"

// InterviewQuestionState は面接セッション内の出題計画と消化状況を表す。
type InterviewQuestionState struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	SessionID         uint      `gorm:"index;not null" json:"session_id"`
	CompanyQuestionID *uint     `gorm:"index" json:"company_question_id,omitempty"`
	Source            string    `gorm:"size:20;not null" json:"source"` // custom | topic | follow_up
	Category          string    `gorm:"size:100" json:"category"`
	QuestionText      string    `gorm:"type:text;not null" json:"question_text"`
	Status            string    `gorm:"size:20;not null;default:'pending'" json:"status"` // pending | asked | answered | skipped
	ParentStateID     *uint     `gorm:"index" json:"parent_state_id,omitempty"`
	Depth             int       `gorm:"default:0" json:"depth"`
	SortOrder         int       `gorm:"default:0" json:"sort_order"`
	IsRequired        bool      `gorm:"default:false" json:"is_required"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
