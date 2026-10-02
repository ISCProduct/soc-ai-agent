package models

import "time"

// スカウトの状態（#1095）。承諾フローの詳細は未確定のため、まずは送信〜辞退まで。
const (
	ScoutStatusSent     = "sent"
	ScoutStatusViewed   = "viewed"
	ScoutStatusAccepted = "accepted"
	ScoutStatusDeclined = "declined"
)

// MaxScoutTemplateTitleLength / MaxScoutTemplateBodyLength は入力上限。
const (
	MaxScoutTemplateTitleLength = 80
	MaxScoutTemplateBodyLength  = 4000
	MaxScoutMessageLength       = 4000
)

// ScoutTemplate は企業ごとのスカウト定型文。
// スキーマは migrations/000042_scouts.up.sql で管理。
type ScoutTemplate struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CompanyID uint      `gorm:"not null;index" json:"company_id"`
	Title     string    `gorm:"type:varchar(80);not null" json:"title"`
	Body      string    `gorm:"type:text;not null" json:"body"`
	CreatedBy uint      `gorm:"not null" json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ScoutTemplate) TableName() string { return "scout_templates" }

// Scout は企業から学生へのスカウト送信履歴。
type Scout struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CompanyID  uint      `gorm:"not null;index" json:"company_id"`
	UserID     uint      `gorm:"not null;index" json:"user_id"`
	TemplateID *uint     `json:"template_id,omitempty"`
	SentBy     uint      `gorm:"not null" json:"sent_by"`
	Message    string    `gorm:"type:text;not null" json:"message"`
	Status     string    `gorm:"type:varchar(16);not null;default:'sent'" json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (Scout) TableName() string { return "scouts" }

// ScoutCompanyBlock は学生が企業からのスカウトを拒否する設定。
type ScoutCompanyBlock struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	CompanyID uint      `gorm:"not null;index" json:"company_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (ScoutCompanyBlock) TableName() string { return "scout_company_blocks" }
