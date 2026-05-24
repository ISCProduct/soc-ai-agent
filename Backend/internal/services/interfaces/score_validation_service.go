package interfaces

import (
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services"
)

// ScoreValidationService スコア精度検証サービスのインターフェース
type ScoreValidationService interface {
	GetCorrelationReport() (*services.CorrelationReport, error)
	GetPhasePrecisionReport() (*services.PhasePrecisionReport, error)
	GetCurrentCalibration() ([]models.ScoreCalibrationWeight, error)
	GetCalibrationHistory(limit int) ([]models.ScoreCalibrationWeight, error)
	RunCalibration() (*services.CalibrationResult, error)
	ListExperiments() ([]string, error)
	CreateVariant(experimentName, variantName, description string, trafficRatio float64) (*models.QuestionVariant, error)
	GetVariantResults(experimentName string) ([]repositories.VariantResultRow, error)
}
