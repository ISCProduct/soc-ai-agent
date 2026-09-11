package mocks

import (
	"Backend/internal/models"
	"context"
	"time"

	"github.com/stretchr/testify/mock"
)

// InterviewVideoRepositoryMock InterviewVideoRepositoryのモック実装
type InterviewVideoRepositoryMock struct {
	mock.Mock
}

func (m *InterviewVideoRepositoryMock) Create(ctx context.Context, v *models.InterviewVideo) error {
	return m.Called(ctx, v).Error(0)
}

func (m *InterviewVideoRepositoryMock) UpdateStatus(ctx context.Context, id uint, status, errorMessage string, driveFileID, driveFileURL string, uploadedAt *time.Time) error {
	return m.Called(ctx, id, status, errorMessage, driveFileID, driveFileURL, uploadedAt).Error(0)
}

func (m *InterviewVideoRepositoryMock) FindBySessionID(ctx context.Context, sessionID uint) ([]models.InterviewVideo, error) {
	args := m.Called(ctx, sessionID)
	if v := args.Get(0); v != nil {
		return v.([]models.InterviewVideo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *InterviewVideoRepositoryMock) FindByID(ctx context.Context, id uint) (*models.InterviewVideo, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*models.InterviewVideo), args.Error(1)
	}
	return nil, args.Error(1)
}
