package models

import "time"

// 掲載申請の状態（#1506）。
const (
	SchoolCompanyApplicationPending  = "pending"
	SchoolCompanyApplicationApproved = "approved"
	SchoolCompanyApplicationRejected = "rejected"
)

// SchoolCompanyApplication は企業→学校への掲載申請。
//
// 企業が「自校の学生に求人・情報を掲載したい」と申請し、学校のキャリア担当
// （学校管理者）が承認する。承認されたら既存の school_company_approvals に
// 行を作り、学生側の絞り込みがそのまま効く（#1505）。
//
// スキーマは migrations/000037_school_company_applications で管理。
type SchoolCompanyApplication struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	SchoolID   uint      `gorm:"not null;index" json:"school_id"`
	CompanyID  uint      `gorm:"not null;index" json:"company_id"`
	Status     string    `gorm:"size:16;not null;default:'pending'" json:"status"`
	AppliedBy  uint      `gorm:"not null" json:"applied_by"`
	ReviewedBy *uint     `json:"reviewed_by,omitempty"`
	Note       string    `gorm:"type:text" json:"note,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (SchoolCompanyApplication) TableName() string {
	return "school_company_applications"
}

// IsPending は承認待ちか。
func (a *SchoolCompanyApplication) IsPending() bool {
	return a != nil && a.Status == SchoolCompanyApplicationPending
}
