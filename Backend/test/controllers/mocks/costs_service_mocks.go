package mocks

import (
	"Backend/internal/services/costs"
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

func (m *APICostServiceMock) GetModelBreakdown(since time.Time) ([]costs.ModelCostSummary, error) {
	args := m.Called(since)
	if v := args.Get(0); v != nil {
		return v.([]costs.ModelCostSummary), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *APICostServiceMock) GetDailyCosts(nDays int) ([]costs.DailyCostSummary, error) {
	args := m.Called(nDays)
	if v := args.Get(0); v != nil {
		return v.([]costs.DailyCostSummary), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *APICostServiceMock) GetMonthlyCosts(nMonths int) ([]costs.MonthlyCostSummary, error) {
	args := m.Called(nMonths)
	if v := args.Get(0); v != nil {
		return v.([]costs.MonthlyCostSummary), args.Error(1)
	}
	return nil, args.Error(1)
}

// RealtimeUsageServiceMock RealtimeUsageServiceのモック実装
type RealtimeUsageServiceMock struct {
	mock.Mock
}

func (m *RealtimeUsageServiceMock) SessionDurationMinutes() int {
	args := m.Called()
	return args.Int(0)
}

func (m *RealtimeUsageServiceMock) CurrentMonthTotalCost() (float64, error) {
	args := m.Called()
	return args.Get(0).(float64), args.Error(1)
}

func (m *RealtimeUsageServiceMock) CurrentActiveCount() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *RealtimeUsageServiceMock) GetUserBreakdown(days int, limit int) ([]costs.RealtimeUserSummary, error) {
	args := m.Called(days, limit)
	if v := args.Get(0); v != nil {
		return v.([]costs.RealtimeUserSummary), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *RealtimeUsageServiceMock) GetDailyUsage(nDays int) ([]costs.RealtimeDailySummary, error) {
	args := m.Called(nDays)
	if v := args.Get(0); v != nil {
		return v.([]costs.RealtimeDailySummary), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *RealtimeUsageServiceMock) GetMonthlyUsage(nMonths int) ([]costs.RealtimeMonthlySummary, error) {
	args := m.Called(nMonths)
	if v := args.Get(0); v != nil {
		return v.([]costs.RealtimeMonthlySummary), args.Error(1)
	}
	return nil, args.Error(1)
}

// CompanySearchBudgetServiceMock CompanySearchBudgetService のモック（#587）
type CompanySearchBudgetServiceMock struct {
	mock.Mock
}

func (m *CompanySearchBudgetServiceMock) Status() (costs.CompanySearchBudgetStatus, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.(costs.CompanySearchBudgetStatus), args.Error(1)
	}
	return costs.CompanySearchBudgetStatus{}, args.Error(1)
}

func (m *CompanySearchBudgetServiceMock) AllowSearch() error {
	args := m.Called()
	return args.Error(0)
}
