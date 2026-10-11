package entity

import "time"

// User ドメインエンティティ（GORM依存なし）
type User struct {
	ID             uint
	OrganizationID uint
	Email          string
	Password       string
	Name           string
	IsGuest        bool
	IsAdmin        bool
	// AdminTokenNotBefore はこれより前に発行された管理者トークンを失効させる(#1155)
	AdminTokenNotBefore      *time.Time
	Role                     string // student / teacher
	TargetLevel              string // 新卒 or 中途
	SchoolName               string
	SchoolID                 *uint
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
	AllowCollectiveInsight   bool
	AllowScoutVisibility     bool
	WithdrawnAt              *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// UserRoleStaff は学校職員（教員・キャリア担当）のロール値。
// models.UserRoleStaff と同じ値（GORM非依存のため定数を重複定義）。
const UserRoleStaff = "staff"

// HasStaffRole は職員ロールを持つか。
func (u *User) HasStaffRole() bool {
	return u != nil && u.Role == UserRoleStaff
}

// CanAccessAdminArea は管理エリア（/admin 配下のAPI）へ入れる主体か。
// 管理者、または職員（is_admin=false の純粋な職員も含む）。
func (u *User) CanAccessAdminArea() bool {
	return u != nil && (u.IsAdmin || u.HasStaffRole())
}

// IsWithdrawn は退会済みかどうか。
func (u *User) IsWithdrawn() bool {
	return u != nil && u.WithdrawnAt != nil
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
