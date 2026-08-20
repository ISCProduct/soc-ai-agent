package mocks

import (
	"Backend/internal/models"
	"time"

	"github.com/stretchr/testify/mock"
)

// InterviewSessionRepositoryMock InterviewSessionRepositoryのモック実装(#982のschool scopeテスト用)
type InterviewSessionRepositoryMock struct {
	mock.Mock
}

func (m *InterviewSessionRepositoryMock) Create(session *models.InterviewSession) error {
	return m.Called(session).Error(0)
}

func (m *InterviewSessionRepositoryMock) FindByID(id uint) (*models.InterviewSession, error) {
	args := m.Called(id)
	if v := args.Get(0); v != nil {
		return v.(*models.InterviewSession), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *InterviewSessionRepositoryMock) Update(session *models.InterviewSession) error {
	return m.Called(session).Error(0)
}

func (m *InterviewSessionRepositoryMock) ListByUser(userID uint, limit int, offset int) ([]models.InterviewSession, error) {
	args := m.Called(userID, limit, offset)
	if v := args.Get(0); v != nil {
		return v.([]models.InterviewSession), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *InterviewSessionRepositoryMock) ListAll(limit int, offset int, schoolID *uint) ([]models.InterviewSession, error) {
	args := m.Called(limit, offset, schoolID)
	if v := args.Get(0); v != nil {
		return v.([]models.InterviewSession), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *InterviewSessionRepositoryMock) ListFinishedByUser(userID uint, limit int) ([]models.InterviewSession, error) {
	args := m.Called(userID, limit)
	if v := args.Get(0); v != nil {
		return v.([]models.InterviewSession), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *InterviewSessionRepositoryMock) CountByUser(userID uint) (int64, error) {
	args := m.Called(userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *InterviewSessionRepositoryMock) CountAll(schoolID *uint) (int64, error) {
	args := m.Called(schoolID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *InterviewSessionRepositoryMock) CountByUserAndDay(userID uint, day time.Time) (int64, error) {
	args := m.Called(userID, day)
	return args.Get(0).(int64), args.Error(1)
}
