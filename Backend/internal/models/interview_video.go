package models

import "time"

// InterviewVideo 面接セッションの録画動画
type InterviewVideo struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	OrganizationID uint       `gorm:"not null;index;column:organization_id" json:"organization_id"`
	SessionID      uint       `gorm:"index;not null" json:"session_id"`
	UserID         uint       `gorm:"index;not null" json:"user_id"`
	DriveFileID    string     `gorm:"size:256" json:"drive_file_id"`
	DriveFileURL   string     `gorm:"size:1024" json:"drive_file_url"`
	FileName       string     `gorm:"size:256" json:"file_name"`
	FileSizeBytes  int64      `json:"file_size_bytes"`
	MimeType       string     `gorm:"size:64" json:"mime_type"`
	Status         string     `gorm:"size:32;not null;default:'pending'" json:"status"` // pending / uploading / done / error
	ErrorMessage   string     `gorm:"type:text" json:"error_message,omitempty"`
	UploadedAt     *time.Time `json:"uploaded_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
