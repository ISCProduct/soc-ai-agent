package models

import "time"

// UserGoogleToken はGoogleカレンダー連携用のOAuthトークンを暗号化して保存するモデル。
type UserGoogleToken struct {
	ID                    uint      `gorm:"primaryKey"                                json:"id"`
	UserID                uint      `gorm:"not null;uniqueIndex"                      json:"user_id"`
	EncryptedAccessToken  string    `gorm:"type:text;not null"                        json:"-"`
	EncryptedRefreshToken string    `gorm:"type:text;not null"                        json:"-"`
	Expiry                time.Time `gorm:"not null"                                  json:"expiry"`
	CalendarID            string    `gorm:"size:255;default:'primary'"                json:"calendar_id"`
	CreatedAt             time.Time `                                                 json:"created_at"`
	UpdatedAt             time.Time `                                                 json:"updated_at"`
}
