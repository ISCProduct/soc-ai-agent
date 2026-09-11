package models

import "time"

// PendingRegistration メールアドレス確認待ちの仮登録
type PendingRegistration struct {
	Token        string    `gorm:"primaryKey;size:255"`
	Email        string    `gorm:"size:255;not null;index"`
	CompanyID    *uint     `gorm:"index" json:"company_id,omitempty"`
	SubmissionID *uint     `gorm:"index" json:"submission_id,omitempty"`
	ExpiresAt    time.Time `gorm:"not null"`
	CreatedAt    time.Time
}
