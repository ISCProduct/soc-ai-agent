package mocks

import (
	"Backend/internal/models"
	"Backend/internal/services"

	"github.com/stretchr/testify/mock"
)

// ProfileRecalculationServiceMock ProfileRecalculationServiceのモック実装
type ProfileRecalculationServiceMock struct {
	mock.Mock
}

func (m *ProfileRecalculationServiceMock) RecalculateAll(minSamples int) ([]*services.RecalculationResult, error) {
	args := m.Called(minSamples)
	if v := args.Get(0); v != nil {
		return v.([]*services.RecalculationResult), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ProfileRecalculationServiceMock) RecalculateCompany(companyID uint, minSamples int) (*services.RecalculationResult, error) {
	args := m.Called(companyID, minSamples)
	if v := args.Get(0); v != nil {
		return v.(*services.RecalculationResult), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ProfileRecalculationServiceMock) Rollback(companyID uint) error {
	args := m.Called(companyID)
	return args.Error(0)
}

func (m *ProfileRecalculationServiceMock) GetHistory(companyID uint) ([]*models.CompanyProfileUpdateHistory, error) {
	args := m.Called(companyID)
	if v := args.Get(0); v != nil {
		return v.([]*models.CompanyProfileUpdateHistory), args.Error(1)
	}
	return nil, args.Error(1)
}
