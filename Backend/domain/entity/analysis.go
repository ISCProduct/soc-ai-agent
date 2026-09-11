package entity

import "time"

// AnalysisPhase 分析フェーズエンティティ
type AnalysisPhase struct {
	ID           uint
	PhaseName    string // job_analysis, interest_analysis, aptitude_analysis, future_analysis
	DisplayName  string
	PhaseOrder   int
	Description  string
	MinQuestions int
	MaxQuestions int
}

// RequiredAnswers フェーズ完了に必要な回答数を返す
func (p *AnalysisPhase) RequiredAnswers() int {
	if p.MaxQuestions > 0 {
		return p.MaxQuestions
	}
	return p.MinQuestions
}

// UserAnalysisProgress ユーザーの分析進捗エンティティ
type UserAnalysisProgress struct {
	ID              uint
	UserID          uint
	SessionID       string
	PhaseID         uint
	Phase           *AnalysisPhase
	QuestionsAsked  int
	ValidAnswers    int
	InvalidAnswers  int
	CompletionScore float64 // 0-100
	IsCompleted     bool
	CompletedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// CompletionPercent 完了率をパーセントで返す
func (p *UserAnalysisProgress) CompletionPercent() float64 {
	if p.Phase == nil {
		return p.CompletionScore
	}
	required := p.Phase.RequiredAnswers()
	if required <= 0 {
		return 0
	}
	pct := float64(p.ValidAnswers) / float64(required) * 100
	if pct > 100 {
		return 100
	}
	return pct
}

// UserWeightScore ユーザーの重みカテゴリ別スコアエンティティ
type UserWeightScore struct {
	ID             uint
	UserID         uint
	SessionID      string
	WeightCategory string
	Score          int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SessionValidation セッションバリデーション状態エンティティ
type SessionValidation struct {
	ID                    uint
	SessionID             string
	InvalidAnswerCount    int
	IsTerminated          bool
	LastInvalidAnswerTime *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
