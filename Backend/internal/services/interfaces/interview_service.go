package interfaces

import (
	"Backend/internal/models"
	"Backend/internal/services"
	"context"
)

type InterviewService interface {
	CreateSession(userID uint, language string, interviewerGender string) (*services.InterviewSessionResponse, error)
	StartSession(userID uint, sessionID uint) (*services.InterviewSessionResponse, error)
	FinishSession(userID uint, sessionID uint) (*services.InterviewSessionResponse, error)
	ListSessions(userID uint, all bool, limit int, offset int) ([]services.InterviewSessionResponse, int64, error)
	GetSessionDetailWithRole(userID uint, sessionID uint, role string) (*services.InterviewDetailResponse, error)
	GetReport(userID uint, sessionID uint) (*models.InterviewReport, error)
	GetPhraseSuggestions(ctx context.Context, userID uint, sessionID uint) ([]services.PhraseSuggestion, error)
	GetTrend(userID uint, limit int) ([]services.InterviewTrendPoint, error)
	SendReportEmail(userID, sessionID uint) error
	SaveUtterance(userID uint, sessionID uint, role string, text string) error
	CreateRealtimeToken(ctx context.Context, userID uint, sessionID uint) (string, error)
	Turn(
		ctx context.Context,
		userID uint,
		sessionID uint,
		audioData []byte,
		history []map[string]string,
		companyName, companyReading, position, companyInfo, companyType string,
		turnCount, remainingSeconds, questionIndex, totalQuestions, questionElapsedSeconds, questionDurationSeconds int,
	) (*services.TurnResult, error)
	StartTurn(
		ctx context.Context,
		userID uint,
		sessionID uint,
		companyName, companyReading, position, companyInfo, companyType string,
		questionIndex, totalQuestions, questionElapsedSeconds, questionDurationSeconds int,
	) (*services.TurnResult, error)
}
