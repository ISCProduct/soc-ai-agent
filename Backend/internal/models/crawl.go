package models

import "time"

// CrawlSource stores schedule and source metadata for automated ingestion.
type CrawlSource struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Name         string     `gorm:"type:varchar(255);not null" json:"name"`
	TargetType   string     `gorm:"type:varchar(50);not null" json:"target_type"`   // company, popular_companies, etc
	SourceType   string     `gorm:"type:varchar(50)" json:"source_type"`            // official, job_site, manual
	SourceURL    string     `gorm:"type:varchar(500)" json:"source_url"`            // crawler target URL
	ScheduleType string     `gorm:"type:varchar(20);not null" json:"schedule_type"` // daily, weekly, monthly
	ScheduleDay  int        `gorm:"default:1" json:"schedule_day"`                  // unused for daily; 0-6 weekly; 1-31 monthly
	ScheduleTime string     `gorm:"type:varchar(10);not null" json:"schedule_time"` // HH:MM
	IsActive     bool       `gorm:"default:true" json:"is_active"`
	LastRunAt    *time.Time `json:"last_run_at,omitempty"`
	NextRunAt    *time.Time `json:"next_run_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// CrawlRun tracks execution history for a source.
type CrawlRun struct {
	ID        uint        `gorm:"primaryKey" json:"id"`
	SourceID  uint        `gorm:"index" json:"source_id"`
	Source    CrawlSource `gorm:"foreignKey:SourceID"`
	Status    string      `gorm:"type:varchar(20);not null" json:"status"` // running, success, failed
	Message   string      `gorm:"type:text" json:"message"`
	StartedAt time.Time   `json:"started_at"`
	EndedAt   *time.Time  `json:"ended_at,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}
