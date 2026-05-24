package mocks

import (
	"Backend/internal/models"
	"Backend/internal/services"

	"github.com/stretchr/testify/mock"
)

// CrawlServiceMock CrawlServiceのモック実装
type CrawlServiceMock struct {
	mock.Mock
}

func (m *CrawlServiceMock) ListSources() ([]models.CrawlSource, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.([]models.CrawlSource), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CrawlServiceMock) ListRuns(sourceID uint, limit int) ([]models.CrawlRun, error) {
	args := m.Called(sourceID, limit)
	if v := args.Get(0); v != nil {
		return v.([]models.CrawlRun), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CrawlServiceMock) CreateSource(payload services.CrawlSourcePayload) (*models.CrawlSource, error) {
	args := m.Called(payload)
	if v := args.Get(0); v != nil {
		return v.(*models.CrawlSource), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CrawlServiceMock) UpdateSource(id uint, payload services.CrawlSourcePayload) (*models.CrawlSource, error) {
	args := m.Called(id, payload)
	if v := args.Get(0); v != nil {
		return v.(*models.CrawlSource), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *CrawlServiceMock) RunSource(id uint) (*models.CrawlRun, error) {
	args := m.Called(id)
	if v := args.Get(0); v != nil {
		return v.(*models.CrawlRun), args.Error(1)
	}
	return nil, args.Error(1)
}
