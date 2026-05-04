package entity

import "time"

// User ドメインエンティティ（GORM依存なし）
type User struct {
	ID                       uint
	Email                    string
	Password                 string
	Name                     string
	IsGuest                  bool
	IsAdmin                  bool
	TargetLevel              string // 新卒 or 中途
	SchoolName               string
	OAuthProvider            string
	OAuthID                  string
	AvatarURL                string
	CertificationsAcquired   string
	CertificationsInProgress string
	EmailVerifiedAt          *time.Time
	EmailVerificationToken   string
	EmailVerificationExpires *time.Time // メール認証トークン有効期限（#330）
	LastLoginAt              *time.Time
	PasswordResetToken       string
	PasswordResetExpiresAt   *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// IsNewGrad 新卒ユーザーかどうか
func (u *User) IsNewGrad() bool {
	return u.TargetLevel == "新卒" || u.TargetLevel == ""
}

// IsEmailVerified メール認証済みかどうか
func (u *User) IsEmailVerified() bool {
	return u.EmailVerifiedAt != nil
}

// HasOAuth OAuth連携済みかどうか
func (u *User) HasOAuth() bool {
	return u.OAuthProvider != "" && u.OAuthID != ""
}
