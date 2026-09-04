package models

import "time"

// CompanyEntrySubmission 人事によるログイン不要の企業・求人投稿（#754）
type CompanyEntrySubmission struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	CompanyID        uint       `gorm:"not null;index" json:"company_id"`
	ContactEmail     string     `gorm:"type:varchar(255);not null;index" json:"contact_email"`
	ContactName      string     `gorm:"type:varchar(255);not null;default:''" json:"contact_name"`
	PrivacyConsentAt time.Time  `gorm:"not null" json:"privacy_consent_at"`
	SourceIP         string     `gorm:"type:varchar(64);not null;default:''" json:"source_ip"`
	Status           string     `gorm:"type:varchar(32);not null;default:'submitted'" json:"status"`     // submitted, claimed, rejected
	EmailStatus      string     `gorm:"type:varchar(32);not null;default:'pending'" json:"email_status"` // pending, sent, failed
	EmailSentAt      *time.Time `json:"email_sent_at,omitempty"`
	EmailLastError   string     `gorm:"type:text" json:"email_last_error,omitempty"`
	EmailAttempts    int        `gorm:"not null;default:0" json:"email_attempts"`
	InviteToken      *string    `gorm:"type:varchar(255)" json:"invite_token,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (CompanyEntrySubmission) TableName() string { return "company_entry_submissions" }

// CompanyOwnership 本登録後の企業クレーム（#754）
type CompanyOwnership struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CompanyID uint      `gorm:"not null;uniqueIndex:uk_company_ownerships_company" json:"company_id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Role      string    `gorm:"type:varchar(32);not null;default:'owner'" json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (CompanyOwnership) TableName() string { return "company_ownerships" }
