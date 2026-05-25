package mocks

import (
	"Backend/internal/models"
	"Backend/internal/services"

	"github.com/stretchr/testify/mock"
)

type CollectiveInsightServiceMock struct {
	mock.Mock
}

func (m *CollectiveInsightServiceMock) GetCollectiveRecommendations(userID uint, sessionID string, excludeCompanyIDs []uint) ([]services.CollectiveRecommendItem, error) {
	args := m.Called(userID, sessionID, excludeCompanyIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]services.CollectiveRecommendItem), args.Error(1)
}

func (m *CollectiveInsightServiceMock) GetTopPassRateCompanies(limit int) ([]models.AnonymizedBehaviorSummary, error) {
	args := m.Called(limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AnonymizedBehaviorSummary), args.Error(1)
}

func (m *CollectiveInsightServiceMock) UpdateConsent(userID uint, allow bool) error {
	return m.Called(userID, allow).Error(0)
}

func (m *CollectiveInsightServiceMock) RecordAction(userID uint, sessionID string, companyID uint, actionType string) error {
	return m.Called(userID, sessionID, companyID, actionType).Error(0)
}

func (m *CollectiveInsightServiceMock) RebuildSummaries() error {
	return m.Called().Error(0)
}
