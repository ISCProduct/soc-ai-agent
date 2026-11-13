package models

import "time"

// GuidanceKindQuestion は学生から教員への質問のスレッド（#2028）。教員が送る案内と違い、
// teacher_user_id は 0（担当校の教員の誰でも答えられる）。
const GuidanceKindQuestion = "question"

// 発言者の役割。
const (
	GuidanceAuthorStudent = "student"
	GuidanceAuthorTeacher = "teacher"
)

// GuidanceMessage は案内のスレッドの発言（#2028）。テーブルは migrations/000070。
// ReadAt は相手側が読んだ日時（学生の発言なら教員が、教員の発言なら学生が読んだ時刻）。
type GuidanceMessage struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	GuidanceID   uint       `gorm:"not null;index" json:"guidance_id"`
	AuthorUserID uint       `gorm:"not null" json:"-"`
	AuthorRole   string     `gorm:"size:16;not null" json:"author_role"`
	Body         string     `gorm:"type:text;not null" json:"body"`
	ReadAt       *time.Time `json:"read_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (GuidanceMessage) TableName() string { return "guidance_messages" }
