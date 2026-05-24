package mocks

import (
	"Backend/internal/services"
	"time"

	"github.com/stretchr/testify/mock"
)

// APICostServiceMock APICostServiceのモック実装
type APICostServiceMock struct {
	mock.Mock
}

func (m *APICostServiceMock) GetCurrentMonthTotal() (float64, error) {
	args := m.Called()
	return args.Get(0).(float64), args.Error(1)
}

func (m *APICostServiceMock) GetModelBreakdown(since time.Time) ([]services.ModelCostSummary, error) {
	args := m.Called(since)
	if v := args.Get(0); v != nil {
		return v.([]services.ModelCostSummary), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *APICostServiceMock) GetDailyCosts(nDays int) ([]services.DailyCostSummary, error) {
	args := m.Called(nDays)
	if v := args.Get(0); v != nil {
		return v.([]services.DailyCostSummary), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *APICostServiceMock) GetMonthlyCosts(nMonths int) ([]services.MonthlyCostSummary, error) {
	args := m.Called(nMonths)
	if v := args.Get(0); v != nil {
		return v.([]services.MonthlyCostSummary), args.Error(1)
	}
	return nil, args.Error(1)
}

// RealtimeUsageServiceMock RealtimeUsageServiceのモック実装
type RealtimeUsageServiceMock struct {
	mock.Mock
}

func (m *RealtimeUsageServiceMock) CurrentMonthTotalCost() (float64, error) {
	args := m.Called()
	return args.Get(0).(float64), args.Error(1)
}

func (m *RealtimeUsageServiceMock) CurrentActiveCount() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *RealtimeUsageServiceMock) GetUserBreakdown(days int, limit int) ([]services.RealtimeUserSummary, error) {
	args := m.Called(days, limit)
	if v := args.Get(0); v != nil {
		return v.([]services.RealtimeUserSummary), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *RealtimeUsageServiceMock) GetDailyUsage(nDays int) ([]services.RealtimeDailySummary, error) {
	args := m.Called(nDays)
	if v := args.Get(0); v != nil {
		return v.([]services.RealtimeDailySummary), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *RealtimeUsageServiceMock) GetMonthlyUsage(nMonths int) ([]services.RealtimeMonthlySummary, error) {
	args := m.Called(nMonths)
	if v := args.Get(0); v != nil {
		return v.([]services.RealtimeMonthlySummary), args.Error(1)
	}
	return nil, args.Error(1)
}
