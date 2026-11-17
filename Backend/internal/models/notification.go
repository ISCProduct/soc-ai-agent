package models

import "time"

// 通知の種類（#2033）。画面の見出しのアイコンや並びに使う。
const (
	NotificationKindApplication = "application" // 選考ステータスの更新（学校・企業が更新したとき）
	NotificationKindScout       = "scout"       // 企業からのスカウト
	NotificationKindGuidance    = "guidance"    // 先生からの案内
	NotificationKindDeadline    = "deadline"    // 予定（締切・面接など）の前日
)

// Notification は個人宛のアプリ内通知（#2033）。テーブルは migrations/000070。
type Notification struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"not null;index" json:"-"`
	Kind      string     `gorm:"size:32;not null" json:"kind"`
	Title     string     `gorm:"size:200;not null" json:"title"`
	Body      string     `gorm:"size:1000;not null;default:''" json:"body"`
	Link      string     `gorm:"size:500;not null;default:''" json:"link"`
	DedupeKey *string    `gorm:"size:191" json:"-"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func (Notification) TableName() string { return "notifications" }
