package models

import "time"

// CompanyStudentTag は企業が学生に付与する自社専用タグ（#1094）。
// company_id で完全に分離され、他社のタグは参照できない。
// スキーマは migrations/000018_scout_student_search.up.sql で管理。
type CompanyStudentTag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CompanyID uint      `gorm:"not null;index" json:"company_id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	TagName   string    `gorm:"type:varchar(64);not null" json:"tag_name"`
	CreatedBy uint      `gorm:"not null" json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (CompanyStudentTag) TableName() string { return "company_student_tags" }

// MaxTagNameLength はタグ名の最大文字数（DBのvarchar(64)に合わせる）。
const MaxTagNameLength = 64
