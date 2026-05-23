package mocks

import (
	"Backend/internal/models"
	"time"

	"github.com/stretchr/testify/mock"
)

type ScheduleServiceMock struct {
	mock.Mock
}

func (m *ScheduleServiceMock) List(userID uint) ([]models.ScheduleEvent, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ScheduleEvent), args.Error(1)
}

func (m *ScheduleServiceMock) ListByRange(userID uint, from, to time.Time) ([]models.ScheduleEvent, error) {
	args := m.Called(userID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ScheduleEvent), args.Error(1)
}

func (m *ScheduleServiceMock) Create(userID uint, companyName, stage, title string, scheduledAt time.Time, notes string) (*models.ScheduleEvent, error) {
	args := m.Called(userID, companyName, stage, title, scheduledAt, notes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ScheduleEvent), args.Error(1)
}

func (m *ScheduleServiceMock) Get(userID, eventID uint) (*models.ScheduleEvent, error) {
	args := m.Called(userID, eventID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ScheduleEvent), args.Error(1)
}

func (m *ScheduleServiceMock) Update(userID, eventID uint, companyName, stage, title string, scheduledAt time.Time, notes string) (*models.ScheduleEvent, error) {
	args := m.Called(userID, eventID, companyName, stage, title, scheduledAt, notes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ScheduleEvent), args.Error(1)
}

func (m *ScheduleServiceMock) Delete(userID, eventID uint) error {
	return m.Called(userID, eventID).Error(0)
}

func (m *ScheduleServiceMock) ExportICS(userID uint) (string, error) {
	args := m.Called(userID)
	return args.String(0), args.Error(1)
}
