package models

import "time"

// InterviewResearchSnapshot は、企業と職種ごとの調査済み質問。
// 期限を過ぎた行は面接で使わない。
type InterviewResearchSnapshot struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	CompanyKey    string    `gorm:"size:191;not null;uniqueIndex:uk_research_company_position" json:"company_key"`
	Position      string    `gorm:"size:191;not null;uniqueIndex:uk_research_company_position" json:"position"`
	QuestionsJSON string    `gorm:"type:text;not null" json:"-"`
	FetchedAt     time.Time `gorm:"not null" json:"fetched_at"`
	ExpiresAt     time.Time `gorm:"not null;index" json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
