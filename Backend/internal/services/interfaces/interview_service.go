package interfaces

import (
	"Backend/internal/models"
	"Backend/internal/services/interview"
	"context"
)

type InterviewService interface {
	CreateSession(userID uint, language string, interviewerGender string) (*interview.InterviewSessionResponse, error)
	StartSession(userID uint, sessionID uint) (*interview.InterviewSessionResponse, error)
	FinishSession(userID uint, sessionID uint) (*interview.InterviewSessionResponse, error)
	ListSessions(userID uint, all bool, limit int, offset int) ([]interview.InterviewSessionResponse, int64, error)
	GetSessionDetailWithRole(userID uint, sessionID uint, role string) (*interview.InterviewDetailResponse, error)
	GetReport(userID uint, sessionID uint) (*models.InterviewReport, error)
	GetPhraseSuggestions(ctx context.Context, userID uint, sessionID uint) ([]interview.PhraseSuggestion, error)
	GetTrend(userID uint, limit int) ([]interview.InterviewTrendPoint, error)
	SendReportEmail(userID, sessionID uint) error
	SaveUtterance(userID uint, sessionID uint, role string, text string) error
	EnsureSessionOwnership(userID uint, sessionID uint) error
	CreateRealtimeToken(ctx context.Context, userID uint, sessionID uint) (string, error)
	Turn(
		ctx context.Context,
		userID uint,
		sessionID uint,
		audioData []byte,
		history []map[string]string,
		companyName, companyReading, position, companyInfo, companyType string,
		companyID uint,
		turnCount, remainingSeconds, questionIndex, totalQuestions, questionElapsedSeconds, questionDurationSeconds int,
	) (*interview.TurnResult, error)
	StartTurn(
		ctx context.Context,
		userID uint,
		sessionID uint,
		companyName, companyReading, position, companyInfo, companyType string,
		companyID uint,
		questionIndex, totalQuestions, questionElapsedSeconds, questionDurationSeconds int,
	) (*interview.TurnResult, error)
}
