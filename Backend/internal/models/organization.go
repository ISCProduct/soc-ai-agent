package models

import "time"

const (
	OrgStatusActive   = "active"
	OrgStatusDisabled = "disabled"

	OrgRoleOwner  = "owner"
	OrgRoleAdmin  = "admin"
	OrgRoleMember = "member"

	// DefaultOrganizationID は既存データを収容するデフォルト組織。
	DefaultOrganizationID uint = 1
	DefaultOrganizationSlug    = "default"
)

// Organization は契約単位（テナント）を表す。
type Organization struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	Slug      string    `gorm:"size:100;uniqueIndex;not null" json:"slug"`
	Status    string    `gorm:"size:20;not null;default:'active';index" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Organization) TableName() string {
	return "organizations"
}

// OrganizationMembership はユーザーと組織の所属関係・ロール。
// 現状はユーザーあたり1組織（user_id UNIQUE）とする。
type OrganizationMembership struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	OrganizationID uint      `gorm:"not null;uniqueIndex:idx_org_membership_user_org;index" json:"organization_id"`
	UserID         uint      `gorm:"not null;uniqueIndex:idx_org_membership_user;uniqueIndex:idx_org_membership_user_org" json:"user_id"`
	Role           string    `gorm:"size:20;not null;default:'member';index" json:"role"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (OrganizationMembership) TableName() string {
	return "organization_memberships"
}

// IsOrgAdminRole は組織内の管理者権限を持つか。
func IsOrgAdminRole(role string) bool {
	return role == OrgRoleOwner || role == OrgRoleAdmin
}
