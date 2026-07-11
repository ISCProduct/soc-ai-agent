package models

import "time"

type JobCategory struct {
	ID           uint          `gorm:"primaryKey" json:"id"`
	ParentID     *uint         `gorm:"index" json:"parent_id,omitempty"`
	Parent       *JobCategory  `gorm:"foreignKey:ParentID" json:"-"`
	Children     []JobCategory `gorm:"foreignKey:ParentID" json:"-"`
	Code         string        `gorm:"uniqueIndex;size:100;not null" json:"code"`
	Name         string        `gorm:"size:255;not null" json:"name"`
	NameEn       string        `gorm:"size:255" json:"name_en"`
	Level        int           `gorm:"default:0" json:"level"`
	Path         string        `gorm:"size:1000" json:"path"`
	Description  string        `gorm:"type:text" json:"description"`
	DisplayOrder int           `gorm:"default:0" json:"display_order"`
	IsActive     bool          `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}
