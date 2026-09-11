package models

import "time"

// CompanyUserRole は企業ポータル内の権限。
const (
	CompanyUserRoleOwner  = "owner"
	CompanyUserRoleMember = "member"
)

// CompanyUser は企業担当者アカウント（#1091）。
// スキーマは migrations/000017_company_users.up.sql と
// migrations/000020_company_user_recovery.up.sql で管理。
//
// 招待トークンとパスワードリセットトークンは平文で持たず、SHA-256 の hex を保存する（#1196）。
type CompanyUser struct {
	ID                     uint       `gorm:"primaryKey" json:"id"`
	CompanyID              uint       `gorm:"not null;index" json:"company_id"`
	Email                  string     `gorm:"type:varchar(255);not null;uniqueIndex" json:"email"`
	Password               string     `gorm:"type:varchar(255);not null;default:''" json:"-"`
	Name                   string     `gorm:"type:varchar(255);not null;default:''" json:"name"`
	Role                   string     `gorm:"type:varchar(32);not null;default:'member'" json:"role"`
	InviteTokenHash        *string    `gorm:"type:varchar(64)" json:"-"`
	InviteExpiresAt        *time.Time `json:"-"`
	PasswordResetTokenHash *string    `gorm:"type:varchar(64)" json:"-"`
	PasswordResetExpiresAt *time.Time `json:"-"`
	// DisabledAt が入っているアカウントはログインもトークン検証も通さない。
	// 行を削除すると company_student_tags の作成者参照が壊れるため、
	// アクセス剥奪はこの列で行う（#1196）。
	DisabledAt *time.Time `json:"disabled_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// Disabled は管理者によって無効化されているか。
func (u *CompanyUser) Disabled() bool {
	return u != nil && u.DisabledAt != nil
}

func (CompanyUser) TableName() string { return "company_users" }

// PasswordSet は招待受諾後にパスワードが設定済みか。
func (u *CompanyUser) PasswordSet() bool {
	return u != nil && u.Password != ""
}
