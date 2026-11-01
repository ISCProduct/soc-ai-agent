package models

import "time"

// 適性検査 第3段: 能力検査（言語・非言語）。スキーマは migrations/000050_ability_assessments。
//
// 性格検査（assessment_*）とテーブルを分けている。能力検査の点数は 10カテゴリへ変換できず
// user_weight_scores へ入れないため、型を共有しないことで混入を構造的に防ぐ（SOCAIAGENT-433）。

const (
	AbilityStatusInProgress = "in_progress"
	AbilityStatusSubmitted  = "submitted"

	// AbilitySectionVerbal / AbilitySectionNumerical は ability_questions.section の値。
	AbilitySectionVerbal    = "verbal"
	AbilitySectionNumerical = "numerical"
)

// AbilitySections は結果に出す順序。section の正典。
var AbilitySections = []string{AbilitySectionVerbal, AbilitySectionNumerical}

type AbilityTemplate struct {
	ID          uint   `gorm:"primaryKey"`
	Slug        string `gorm:"size:64"`
	Title       string `gorm:"size:255"`
	Description string
	Version     int
	// TimeLimitSeconds は受検の制限時間。NULL は無制限。
	TimeLimitSeconds *int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (AbilityTemplate) TableName() string { return "ability_templates" }

// AbilityQuestion は4択問題。CorrectChoice はクライアントへ渡してはいけない。
// レスポンスには専用のビュー型（AbilityQuestionView）を使い、この型を直接返さない。
type AbilityQuestion struct {
	ID              uint `gorm:"primaryKey"`
	TemplateID      uint
	TemplateVersion int
	Position        int
	Section         string `gorm:"size:20"`
	Body            string
	// Choices は選択肢の JSON 配列。表示順がそのまま添字になる。
	Choices string `gorm:"type:json"`
	// CorrectChoice は Choices の 0 始まりの添字。
	CorrectChoice int
	CreatedAt     time.Time
}

func (AbilityQuestion) TableName() string { return "ability_questions" }

type AbilitySession struct {
	ID              uint   `gorm:"primaryKey"`
	PublicID        string `gorm:"size:36"`
	UserID          uint
	TemplateID      uint
	TemplateVersion int
	Status          string `gorm:"size:20"`
	// DeadlineAt は開始時に凍結した締め切り。NULL は無制限。判定はサーバだけが行う。
	DeadlineAt  *time.Time
	SubmittedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (AbilitySession) TableName() string { return "ability_sessions" }

type AbilityAnswer struct {
	ID         uint `gorm:"primaryKey"`
	SessionID  uint
	QuestionID uint
	// Choice は選んだ選択肢の 0 始まりの添字。
	Choice    int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (AbilityAnswer) TableName() string { return "ability_answers" }

type AbilityResult struct {
	ID        uint `gorm:"primaryKey"`
	SessionID uint
	UserID    uint
	// SectionScores は section -> {"correct": n, "total": m} の JSON。
	SectionScores string `gorm:"type:json"`
	CreatedAt     time.Time
}

func (AbilityResult) TableName() string { return "ability_results" }
