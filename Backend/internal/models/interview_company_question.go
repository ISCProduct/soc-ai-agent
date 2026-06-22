package models

import "time"

// InterviewCompanyQuestion 企業別面接カスタム質問
type InterviewCompanyQuestion struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	CompanyID    uint      `gorm:"index;not null" json:"company_id"`
	Position     string    `gorm:"size:100" json:"position"` // 空欄=全職種共通
	Category     string    `gorm:"size:100;not null" json:"category"`
	QuestionText string    `gorm:"type:text;not null" json:"question_text"`
	Priority     int       `gorm:"default:0" json:"priority"` // 小さい数値が先
	IsRequired   bool      `gorm:"default:false" json:"is_required"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
