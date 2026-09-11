package models

import "time"

// ReleaseNoteAudienceAll は全ユーザー向け（役割を問わず表示）。
const ReleaseNoteAudienceAll = "all"

// ReleaseNote GitHub PRからAI要約して生成する更新情報（#861, #966）
type ReleaseNote struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PRNumber  uint      `gorm:"not null;uniqueIndex:uq_release_notes_pr_number" json:"pr_number"`
	Title     string    `gorm:"type:varchar(255);not null" json:"title"`
	Summary   string    `gorm:"type:text;not null" json:"summary"`
	Audience  string    `gorm:"type:varchar(20);not null;default:all" json:"audience"`
	MergedAt  time.Time `gorm:"not null;index" json:"merged_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (ReleaseNote) TableName() string { return "release_notes" }
