package models

import "time"

// ユーザーロール。role 列（migrations/000001）に入る値。
//
// staff は学校職員（教員・キャリア担当）。is_admin とは独立で、is_admin でない
// 純粋な職員も表現できる。職員は担当校（admin_school_memberships）でスコープされ、
// 担当校を持たない職員は何も見えない（fail-close）。「無制限＝全校閲覧」は
// is_admin のときだけ成立する（school.ResolveAccess）。
const (
	UserRoleStudent = "student"
	UserRoleStaff   = "staff"
)

// User ユーザー情報
type User struct {
	ID             uint   `gorm:"primaryKey"`
	OrganizationID uint   `gorm:"not null;index;default:1;column:organization_id" json:"organization_id"`
	Email          string `gorm:"size:255;uniqueIndex;not null"`
	Password       string `gorm:"size:255"` // ハッシュ化されたパスワード (OAuth時は空)
	Name           string `gorm:"size:100"`
	IsGuest        bool   `gorm:"default:false"`                                     // ゲストユーザーフラグ
	Role           string `gorm:"size:20;default:'student'" json:"role"`             // ユーザーロール: student / teacher
	TargetLevel    string `gorm:"size:20;default:'新卒'"`                              // 新卒 or 中途
	SchoolName     string `gorm:"size:255;column:school_name"`                       // 学校名(自由記述)
	SchoolID       *uint  `gorm:"column:school_id;index" json:"school_id,omitempty"` // 個別校(構造化、既存データはNULLのまま)
	IsAdmin        bool   `gorm:"default:false" json:"is_admin"`                     // 管理者フラグ
	// AdminTokenNotBefore はこれより前に発行された管理者トークンを失効させる(#1155)。NULLは失効なし。
	AdminTokenNotBefore      *time.Time `gorm:"column:admin_token_not_before" json:"-"`
	OAuthProvider            string     `gorm:"size:50;column:oauth_provider"`                           // OAuth提供者 (google, github, など)
	OAuthID                  string     `gorm:"size:255;index;column:oauth_id"`                          // OAuth提供者のユーザーID
	AvatarURL                string     `gorm:"size:500;column:avatar_url"`                              // プロフィール画像URL
	CertificationsAcquired   string     `gorm:"type:text;column:certifications_acquired"`                // 取得資格
	CertificationsInProgress string     `gorm:"type:text;column:certifications_in_progress"`             // 勉強中の資格
	EmailVerifiedAt          *time.Time `gorm:"column:email_verified_at"`                                // メール認証日時
	EmailVerificationToken   string     `gorm:"size:255;column:email_verification_token"`                // メール認証トークン
	EmailVerificationExpires *time.Time `gorm:"column:email_verification_expires"`                       // メール認証トークン有効期限（#330）
	LastLoginAt              *time.Time `gorm:"column:last_login_at"`                                    // 最終ログイン日時
	PasswordResetToken       string     `gorm:"size:255;column:password_reset_token"`                    // パスワードリセットトークン
	PasswordResetExpiresAt   *time.Time `gorm:"column:password_reset_expires_at"`                        // パスワードリセットトークン有効期限
	AllowCollectiveInsight   bool       `gorm:"default:true;column:allow_collective_insight"`            // 集合知レコメンドへの参加同意
	AllowScoutVisibility     bool       `gorm:"default:false;column:allow_scout_visibility"`             // 企業スカウト向け分析データ公開同意（#1096）
	WithdrawnAt              *time.Time `gorm:"index;column:withdrawn_at" json:"withdrawn_at,omitempty"` // 退会日時（論理削除）
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// HasStaffRole は職員ロールを持つか。
func (u *User) HasStaffRole() bool {
	return u != nil && u.Role == UserRoleStaff
}

// CanAccessAdminArea は管理エリア（/admin 配下のAPI）へ入れる主体か。
// 管理者、または職員。純粋な職員（is_admin=false）もここを通す。
func (u *User) CanAccessAdminArea() bool {
	return u != nil && (u.IsAdmin || u.HasStaffRole())
}

// IsWithdrawn は退会済み（猶予期間中含む）かどうか。
func (u *User) IsWithdrawn() bool {
	return u != nil && u.WithdrawnAt != nil
}
