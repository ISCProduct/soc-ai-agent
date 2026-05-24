package mocks

import (
	"Backend/internal/models"
	"Backend/internal/services"

	"github.com/stretchr/testify/mock"
)

// ScraperSessionServiceMock ScraperSessionServiceのモック実装
type ScraperSessionServiceMock struct {
	mock.Mock
}

func (m *ScraperSessionServiceMock) List() ([]models.ScraperSession, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.([]models.ScraperSession), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ScraperSessionServiceMock) Upsert(payload services.ScraperSessionPayload) (*models.ScraperSession, error) {
	args := m.Called(payload)
	if v := args.Get(0); v != nil {
		return v.(*models.ScraperSession), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ScraperSessionServiceMock) Delete(siteKey string) error {
	args := m.Called(siteKey)
	return args.Error(0)
}
