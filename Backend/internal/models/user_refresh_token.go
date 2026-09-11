package models

import "time"

// UserRefreshToken はリフレッシュトークンを管理する (#616)
// トークン本体は保存せず SHA-256 ハッシュのみ保持する。
// スキーマは migrations/000002_create_user_refresh_tokens.up.sql で管理。
type UserRefreshToken struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	TokenHash string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	ExpiresAt time.Time  `gorm:"not null;index" json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
