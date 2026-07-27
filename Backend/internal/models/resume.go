package models

import "time"

type ResumeDocument struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	OrganizationID   uint      `gorm:"not null;index;column:organization_id" json:"organization_id"`
	UserID           uint      `gorm:"not null;index" json:"user_id"`
	SessionID        string    `gorm:"size:100;index" json:"session_id"`
	SourceType       string    `gorm:"size:50;not null" json:"source_type"` // pdf, docx, google_docs
	SourceURL        string    `gorm:"type:text" json:"source_url,omitempty"`
	OriginalFilename string    `gorm:"size:255" json:"original_filename"`
	StoredPath       string    `gorm:"type:text;not null" json:"stored_path"`
	NormalizedPath   string    `gorm:"type:text" json:"normalized_path,omitempty"`
	AnnotatedPath    string    `gorm:"type:text" json:"annotated_path,omitempty"`
	Status           string    `gorm:"size:50;default:'uploaded'" json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ResumeTextBlock struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	DocumentID uint      `gorm:"not null;index" json:"document_id"`
	PageNumber int       `gorm:"not null" json:"page_number"`
	BlockIndex int       `gorm:"not null" json:"block_index"`
	Text       string    `gorm:"type:text;not null" json:"text"`
	BBox       string    `gorm:"type:json;not null" json:"bbox"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ResumeReview struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	DocumentID uint      `gorm:"not null;index" json:"document_id"`
	Score      int       `gorm:"not null;default:0" json:"score"`
	Summary    string    `gorm:"type:text" json:"summary"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ResumeReviewItem struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ReviewID   uint      `gorm:"not null;index" json:"review_id"`
	PageNumber int       `gorm:"not null" json:"page_number"`
	BBox       string    `gorm:"type:json;not null" json:"bbox"`
	Severity   string    `gorm:"size:20;not null" json:"severity"` // info, warning, critical
	Message    string    `gorm:"type:text;not null" json:"message"`
	Suggestion string    `gorm:"type:text" json:"suggestion,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
