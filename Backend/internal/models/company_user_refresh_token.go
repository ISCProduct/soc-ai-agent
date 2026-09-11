package models

import "time"

// CompanyUserRefreshToken は企業ユーザーのリフレッシュトークン（#1091）。
type CompanyUserRefreshToken struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	CompanyUserID uint       `gorm:"not null;index" json:"company_user_id"`
	TokenHash     string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	ExpiresAt     time.Time  `gorm:"not null;index" json:"expires_at"`
	RevokedAt     *time.Time `json:"revoked_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (CompanyUserRefreshToken) TableName() string { return "company_user_refresh_tokens" }
