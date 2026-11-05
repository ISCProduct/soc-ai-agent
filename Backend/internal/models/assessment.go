package models

import "time"

// 適性検査 第1段（#1678）。スキーマは migrations/000048_assessments。

const (
	AssessmentStatusInProgress = "in_progress"
	AssessmentStatusSubmitted  = "submitted"

	// ScoreSourceChat / ScoreSourceAssessment は user_weight_scores.source の値。
	ScoreSourceChat       = "chat"
	ScoreSourceAssessment = "assessment"
)

type AssessmentTemplate struct {
	ID       uint  `gorm:"primaryKey"`
	SchoolID *uint // NULL は運営標準
	// OrganizationID は学校独自テンプレートのテナント境界（#1675）。運営標準は NULL。
	OrganizationID *uint
	Slug           string `gorm:"size:64"`
	Title          string `gorm:"size:255"`
	Description    string
	Version        int
	IsStandard     bool
	DueAt          *time.Time // 受検期限。NULL は期限なし（#1675）
	CreatedBy      *uint      // 作成した職員の users.id（監査用）
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type AssessmentQuestion struct {
	ID              uint `gorm:"primaryKey"`
	TemplateID      uint
	TemplateVersion int
	Position        int
	Body            string
	WeightCategory  string `gorm:"size:100"`
	ReverseScored   bool
	CreatedAt       time.Time
}

type AssessmentSession struct {
	ID              uint   `gorm:"primaryKey"`
	PublicID        string `gorm:"size:36"`
	UserID          uint
	TemplateID      uint
	TemplateVersion int
	Status          string `gorm:"size:20"`
	SubmittedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type AssessmentAnswer struct {
	ID         uint `gorm:"primaryKey"`
	SessionID  uint
	QuestionID uint
	Value      int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type AssessmentResult struct {
	ID             uint `gorm:"primaryKey"`
	SessionID      uint
	UserID         uint
	CategoryScores string `gorm:"type:json"` // カテゴリ名 -> 0〜100
	CreatedAt      time.Time
}
