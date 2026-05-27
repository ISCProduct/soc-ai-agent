package mocks

import (
	"Backend/internal/models"
	"Backend/internal/services"

	"github.com/stretchr/testify/mock"
)

type CrossFeatureServiceMock struct {
	mock.Mock
}

func (m *CrossFeatureServiceMock) BuildIntegratedProfile(userID uint, chatSessionID string, interviewCount int, resumeReviewDone bool) (*services.UserIntegratedProfile, error) {
	args := m.Called(userID, chatSessionID, interviewCount, resumeReviewDone)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.UserIntegratedProfile), args.Error(1)
}

type InterviewSessionCounterMock struct {
	mock.Mock
}

func (m *InterviewSessionCounterMock) CountByUser(userID uint) (int64, error) {
	args := m.Called(userID)
	return args.Get(0).(int64), args.Error(1)
}

type ResumeDocumentFinderMock struct {
	mock.Mock
}

func (m *ResumeDocumentFinderMock) FindDocumentsByUserID(userID uint) ([]models.ResumeDocument, error) {
	args := m.Called(userID)
	return args.Get(0).([]models.ResumeDocument), args.Error(1)
}
