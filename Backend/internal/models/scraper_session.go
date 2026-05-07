package models

import "time"

// ScraperSession スクレイピング用ログインセッションCookieの管理
type ScraperSession struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	SiteKey   string `gorm:"type:varchar(50);not null;uniqueIndex" json:"site_key"` // "mynavi", "rikunabi" etc.
	Cookies   string `gorm:"type:text;not null" json:"cookies"`                    // JSON: [{name,value,domain,path}]
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// ScraperCookie Cookie1件分の構造体
type ScraperCookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain,omitempty"`
	Path   string `json:"path,omitempty"`
}
