package models

import "time"

// CompanyPopularityRecord stores evidence-based popularity notes from external sources.
type CompanyPopularityRecord struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	CompanyID    uint      `gorm:"not null;index" json:"company_id"`
	Company      Company   `gorm:"foreignKey:CompanyID"`
	SourceName   string    `gorm:"type:varchar(100);not null" json:"source_name"`
	SourceURL    string    `gorm:"type:varchar(500)" json:"source_url"`
	EvidenceText string    `gorm:"type:text" json:"evidence_text"`
	Summary      string    `gorm:"type:text" json:"summary"`
	Rank         *int      `json:"rank,omitempty"`
	FetchedAt    time.Time `json:"fetched_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// GraduateEmployment stores alumni employment reports.
type GraduateEmployment struct {
	ID             uint                `gorm:"primaryKey" json:"id"`
	CompanyID      uint                `gorm:"not null;index" json:"company_id"`
	Company        Company             `gorm:"foreignKey:CompanyID"`
	JobPositionID  *uint               `gorm:"index" json:"job_position_id,omitempty"`
	JobPosition    *CompanyJobPosition `gorm:"foreignKey:JobPositionID" json:"job_position,omitempty"`
	GraduateName   string              `gorm:"type:varchar(255)" json:"graduate_name"`
	GraduationYear int                 `json:"graduation_year"`
	SchoolName     string              `gorm:"type:varchar(255)" json:"school_name"`
	SchoolID       *uint               `gorm:"index" json:"school_id,omitempty"`
	Department     string              `gorm:"type:varchar(255)" json:"department"`
	HiredAt        *time.Time          `json:"hired_at,omitempty"`
	Note           string              `gorm:"type:text" json:"note"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
	DeletedAt      *time.Time          `gorm:"index" json:"deleted_at,omitempty"`
}
