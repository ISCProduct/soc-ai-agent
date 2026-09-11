package interfaces

import (
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/admin"
)

// ScoreValidationService スコア精度検証サービスのインターフェース
type ScoreValidationService interface {
	GetCorrelationReport() (*admin.CorrelationReport, error)
	GetPhasePrecisionReport() (*admin.PhasePrecisionReport, error)
	GetCurrentCalibration() ([]models.ScoreCalibrationWeight, error)
	GetCalibrationHistory(limit int) ([]models.ScoreCalibrationWeight, error)
	RunCalibration() (*admin.CalibrationResult, error)
	ListExperiments() ([]string, error)
	ListAllVariants() ([]models.QuestionVariant, error)
	CreateVariant(experimentName, variantName, description string, trafficRatio float64) (*models.QuestionVariant, error)
	GetVariantResults(experimentName string) ([]repositories.VariantResultRow, error)
}
