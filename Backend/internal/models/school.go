package models

import "time"

const (
	SchoolStatusActive   = "active"
	SchoolStatusDisabled = "disabled"

	// DefaultInterviewVideoRetentionDays は、学校が期間を選ぶまでの面接動画の保持日数（SOCAIAGENT-671）。
	DefaultInterviewVideoRetentionDays = 90
)

// School は学園(Organization)配下の個別校を表す(例: 情報科学専門学校 / 横浜デジタルアーツ専門学校)。
type School struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	OrganizationID uint   `gorm:"not null;index" json:"organization_id"`
	Name           string `gorm:"size:255;not null" json:"name"`
	Status         string `gorm:"size:20;not null;default:'active';index" json:"status"`
	// InterviewVideoRetentionDays は面接動画を残す日数。nil は期限なし。未設定の学校は 90。
	InterviewVideoRetentionDays *int      `gorm:"column:interview_video_retention_days" json:"interview_video_retention_days"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
}

func (School) TableName() string {
	return "schools"
}

// AdminSchoolMembership は管理者(先生)が担当する学校。
// 0件の管理者は「システム管理者」として学校による絞り込みを受けない。
type AdminSchoolMembership struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;uniqueIndex:idx_admin_school_memberships_user_school" json:"user_id"`
	SchoolID  uint      `gorm:"not null;uniqueIndex:idx_admin_school_memberships_user_school;index" json:"school_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AdminSchoolMembership) TableName() string {
	return "admin_school_memberships"
}

// SchoolCompanyApproval はその学校向けに先生が承認した企業(掲載承認リスト)。
type SchoolCompanyApproval struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SchoolID  uint      `gorm:"not null;uniqueIndex:idx_school_company_approvals" json:"school_id"`
	CompanyID uint      `gorm:"not null;uniqueIndex:idx_school_company_approvals;index" json:"company_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SchoolCompanyApproval) TableName() string {
	return "school_company_approvals"
}
