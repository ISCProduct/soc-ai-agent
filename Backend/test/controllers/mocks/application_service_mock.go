package mocks

import (
	"Backend/domain/entity"

	"github.com/stretchr/testify/mock"
)

// ApplicationServiceMock ApplicationServiceインターフェースのモック実装
type ApplicationServiceMock struct {
	mock.Mock
}

func (m *ApplicationServiceMock) Apply(userID, companyID, matchID uint) (*entity.UserApplicationStatus, error) {
	args := m.Called(userID, companyID, matchID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.UserApplicationStatus), args.Error(1)
}

func (m *ApplicationServiceMock) UpdateStatus(applicationID uint, userID uint, status, notes string) (*entity.UserApplicationStatus, error) {
	args := m.Called(applicationID, userID, status, notes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.UserApplicationStatus), args.Error(1)
}

func (m *ApplicationServiceMock) GetApplicationsByUser(userID uint) ([]*entity.UserApplicationStatus, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.UserApplicationStatus), args.Error(1)
}

func (m *ApplicationServiceMock) GetCorrelation(companyID uint) ([]map[string]interface{}, error) {
	args := m.Called(companyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]map[string]interface{}), args.Error(1)
}
