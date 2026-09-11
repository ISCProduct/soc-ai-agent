package entity

import "time"

// InterviewSession 面接セッションエンティティ
type InterviewSession struct {
	ID               uint
	UserID           uint
	Status           string // pending, in_progress, completed, failed
	Language         string // ja, en
	StartedAt        *time.Time
	EndedAt          *time.Time
	EstimatedCostUSD float64
	TemplateVersion  string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IsActive セッションが進行中かどうか
func (s *InterviewSession) IsActive() bool {
	return s.Status == "in_progress"
}

// IsFinished セッションが終了しているかどうか
func (s *InterviewSession) IsFinished() bool {
	return s.Status == "completed" || s.Status == "failed"
}

// Duration 面接時間を返す（終了している場合のみ）
func (s *InterviewSession) Duration() *time.Duration {
	if s.StartedAt == nil || s.EndedAt == nil {
		return nil
	}
	d := s.EndedAt.Sub(*s.StartedAt)
	return &d
}

// InterviewReport 面接レポートエンティティ
type InterviewReport struct {
	SessionID    uint
	SummaryText  string
	ScoresJSON   string
	EvidenceJSON string
}
