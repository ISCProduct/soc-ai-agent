package models

import "time"

// ReleaseNote GitHub PRからAI要約して生成する更新情報（#861）
type ReleaseNote struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PRNumber  uint      `gorm:"not null;uniqueIndex:uq_release_notes_pr_number" json:"pr_number"`
	Title     string    `gorm:"type:varchar(255);not null" json:"title"`
	Summary   string    `gorm:"type:text;not null" json:"summary"`
	MergedAt  time.Time `gorm:"not null;index" json:"merged_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (ReleaseNote) TableName() string { return "release_notes" }
