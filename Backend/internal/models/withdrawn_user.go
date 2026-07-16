package models

import "time"

// WithdrawnUser は退会済みユーザーの猶予期間管理用レコード。
// 退会直後は論理削除のみとし、PurgeAfter 経過後に物理削除する（#613）。
type WithdrawnUser struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	UserID       uint       `gorm:"uniqueIndex;not null" json:"user_id"`
	EmailHash    string     `gorm:"size:64;index;not null" json:"email_hash"` // SHA-256(lowercase email)
	EmailMasked  string     `gorm:"size:255" json:"email_masked"`             // 例: u***@example.com
	Reason       string     `gorm:"size:20;not null" json:"reason"`           // self | admin
	ActorEmail   string     `gorm:"size:255" json:"actor_email,omitempty"`
	S3ObjectKeys string     `gorm:"type:text" json:"s3_object_keys"` // JSON 配列
	WithdrawnAt  time.Time  `gorm:"not null;index" json:"withdrawn_at"`
	PurgeAfter   time.Time  `gorm:"not null;index" json:"purge_after"`
	PurgedAt     *time.Time `json:"purged_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (WithdrawnUser) TableName() string {
	return "withdrawn_users"
}
