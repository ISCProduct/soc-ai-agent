package mocks

import (
	"Backend/internal/models"
	"Backend/internal/services"
	"context"

	"github.com/stretchr/testify/mock"
)

type InterviewServiceMock struct {
	mock.Mock
}

func (m *InterviewServiceMock) CreateSession(userID uint, language string, interviewerGender string) (*services.InterviewSessionResponse, error) {
	args := m.Called(userID, language, interviewerGender)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.InterviewSessionResponse), args.Error(1)
}

func (m *InterviewServiceMock) StartSession(userID uint, sessionID uint) (*services.InterviewSessionResponse, error) {
	args := m.Called(userID, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.InterviewSessionResponse), args.Error(1)
}

func (m *InterviewServiceMock) FinishSession(userID uint, sessionID uint) (*services.InterviewSessionResponse, error) {
	args := m.Called(userID, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.InterviewSessionResponse), args.Error(1)
}

func (m *InterviewServiceMock) ListSessions(userID uint, all bool, limit int, offset int) ([]services.InterviewSessionResponse, int64, error) {
	args := m.Called(userID, all, limit, offset)
	return args.Get(0).([]services.InterviewSessionResponse), args.Get(1).(int64), args.Error(2)
}

func (m *InterviewServiceMock) GetSessionDetailWithRole(userID uint, sessionID uint, role string) (*services.InterviewDetailResponse, error) {
	args := m.Called(userID, sessionID, role)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.InterviewDetailResponse), args.Error(1)
}

func (m *InterviewServiceMock) GetReport(userID uint, sessionID uint) (*models.InterviewReport, error) {
	args := m.Called(userID, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.InterviewReport), args.Error(1)
}

func (m *InterviewServiceMock) GetPhraseSuggestions(ctx context.Context, userID uint, sessionID uint) ([]services.PhraseSuggestion, error) {
	args := m.Called(ctx, userID, sessionID)
	return args.Get(0).([]services.PhraseSuggestion), args.Error(1)
}

func (m *InterviewServiceMock) GetTrend(userID uint, limit int) ([]services.InterviewTrendPoint, error) {
	args := m.Called(userID, limit)
	return args.Get(0).([]services.InterviewTrendPoint), args.Error(1)
}

func (m *InterviewServiceMock) SendReportEmail(userID, sessionID uint) error {
	return m.Called(userID, sessionID).Error(0)
}

func (m *InterviewServiceMock) SaveUtterance(userID uint, sessionID uint, role string, text string) error {
	return m.Called(userID, sessionID, role, text).Error(0)
}

func (m *InterviewServiceMock) CreateRealtimeToken(ctx context.Context, userID uint, sessionID uint) (string, error) {
	args := m.Called(ctx, userID, sessionID)
	return args.String(0), args.Error(1)
}

func (m *InterviewServiceMock) Turn(
	ctx context.Context,
	userID uint,
	sessionID uint,
	audioData []byte,
	history []map[string]string,
	companyName, companyReading, position, companyInfo, companyType string,
	turnCount, remainingSeconds, questionIndex, totalQuestions, questionElapsedSeconds, questionDurationSeconds int,
) (*services.TurnResult, error) {
	args := m.Called(ctx, userID, sessionID, audioData, history,
		companyName, companyReading, position, companyInfo, companyType,
		turnCount, remainingSeconds, questionIndex, totalQuestions, questionElapsedSeconds, questionDurationSeconds)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.TurnResult), args.Error(1)
}

func (m *InterviewServiceMock) StartTurn(
	ctx context.Context,
	userID uint,
	sessionID uint,
	companyName, companyReading, position, companyInfo, companyType string,
	questionIndex, totalQuestions, questionElapsedSeconds, questionDurationSeconds int,
) (*services.TurnResult, error) {
	args := m.Called(ctx, userID, sessionID,
		companyName, companyReading, position, companyInfo, companyType,
		questionIndex, totalQuestions, questionElapsedSeconds, questionDurationSeconds)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.TurnResult), args.Error(1)
}
