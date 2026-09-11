package mocks

import (
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/admin"

	"github.com/stretchr/testify/mock"
)

// ScoreValidationServiceMock ScoreValidationServiceのモック実装
type ScoreValidationServiceMock struct {
	mock.Mock
}

func (m *ScoreValidationServiceMock) GetCorrelationReport() (*admin.CorrelationReport, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.(*admin.CorrelationReport), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ScoreValidationServiceMock) GetPhasePrecisionReport() (*admin.PhasePrecisionReport, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.(*admin.PhasePrecisionReport), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ScoreValidationServiceMock) GetCurrentCalibration() ([]models.ScoreCalibrationWeight, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.([]models.ScoreCalibrationWeight), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ScoreValidationServiceMock) GetCalibrationHistory(limit int) ([]models.ScoreCalibrationWeight, error) {
	args := m.Called(limit)
	if v := args.Get(0); v != nil {
		return v.([]models.ScoreCalibrationWeight), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ScoreValidationServiceMock) RunCalibration() (*admin.CalibrationResult, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.(*admin.CalibrationResult), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ScoreValidationServiceMock) ListExperiments() ([]string, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.([]string), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ScoreValidationServiceMock) ListAllVariants() ([]models.QuestionVariant, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.([]models.QuestionVariant), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ScoreValidationServiceMock) CreateVariant(experimentName, variantName, description string, trafficRatio float64) (*models.QuestionVariant, error) {
	args := m.Called(experimentName, variantName, description, trafficRatio)
	if v := args.Get(0); v != nil {
		return v.(*models.QuestionVariant), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ScoreValidationServiceMock) GetVariantResults(experimentName string) ([]repositories.VariantResultRow, error) {
	args := m.Called(experimentName)
	if v := args.Get(0); v != nil {
		return v.([]repositories.VariantResultRow), args.Error(1)
	}
	return nil, args.Error(1)
}
