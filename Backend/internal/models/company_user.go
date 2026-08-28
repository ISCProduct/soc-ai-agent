package models

import "time"

// CompanyUserRole は企業ポータル内の権限。
const (
	CompanyUserRoleOwner  = "owner"
	CompanyUserRoleMember = "member"
)

// CompanyUser は企業担当者アカウント（#1091）。
// スキーマは migrations/000016_company_users.up.sql で管理。
type CompanyUser struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	CompanyID       uint       `gorm:"not null;index" json:"company_id"`
	Email           string     `gorm:"type:varchar(255);not null;uniqueIndex" json:"email"`
	Password        string     `gorm:"type:varchar(255);not null;default:''" json:"-"`
	Name            string     `gorm:"type:varchar(255);not null;default:''" json:"name"`
	Role            string     `gorm:"type:varchar(32);not null;default:'member'" json:"role"`
	InviteToken     *string    `gorm:"type:varchar(255)" json:"-"`
	InviteExpiresAt *time.Time `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (CompanyUser) TableName() string { return "company_users" }

// PasswordSet は招待受諾後にパスワードが設定済みか。
func (u *CompanyUser) PasswordSet() bool {
	return u != nil && u.Password != ""
}
