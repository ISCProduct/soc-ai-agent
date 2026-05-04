package models

import "time"

// User ユーザー情報
type User struct {
	ID                       uint   `gorm:"primaryKey"`
	Email                    string `gorm:"size:255;uniqueIndex;not null"`
	Password                 string `gorm:"size:255"` // ハッシュ化されたパスワード (OAuth時は空)
	Name                     string `gorm:"size:100"`
	IsGuest                  bool   `gorm:"default:false"`                               // ゲストユーザーフラグ
	Role                     string `gorm:"size:20;default:'student'" json:"role"`       // ユーザーロール: student / teacher
	TargetLevel              string `gorm:"size:20;default:'新卒'"`                        // 新卒 or 中途
	SchoolName               string `gorm:"size:255;column:school_name"`                 // 学校名
	IsAdmin                  bool   `gorm:"default:false" json:"is_admin"`               // 管理者フラグ
	OAuthProvider            string `gorm:"size:50;column:oauth_provider"`               // OAuth提供者 (google, github, など)
	OAuthID                  string `gorm:"size:255;index;column:oauth_id"`              // OAuth提供者のユーザーID
	AvatarURL                string `gorm:"size:500;column:avatar_url"`                  // プロフィール画像URL
	CertificationsAcquired   string     `gorm:"type:text;column:certifications_acquired"`    // 取得資格
	CertificationsInProgress string     `gorm:"type:text;column:certifications_in_progress"` // 勉強中の資格
	EmailVerifiedAt          *time.Time `gorm:"column:email_verified_at"`                    // メール認証日時
	EmailVerificationToken   string     `gorm:"size:255;column:email_verification_token"`    // メール認証トークン
	EmailVerificationExpires *time.Time `gorm:"column:email_verification_expires"`           // メール認証トークン有効期限（#330）
	LastLoginAt              *time.Time `gorm:"column:last_login_at"`                        // 最終ログイン日時
	PasswordResetToken       string     `gorm:"size:255;column:password_reset_token"`        // パスワードリセットトークン
	PasswordResetExpiresAt   *time.Time `gorm:"column:password_reset_expires_at"`            // パスワードリセットトークン有効期限
	AllowCollectiveInsight   bool       `gorm:"default:true;column:allow_collective_insight"` // 集合知レコメンドへの参加同意
	CreatedAt                time.Time
	UpdatedAt                time.Time
}
