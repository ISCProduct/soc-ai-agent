package interfaces

import (
	"Backend/internal/models"
	"Backend/internal/services/flywheel"
)

// ProfileRecalculationService プロファイル再計算サービスのインターフェース
type ProfileRecalculationService interface {
	RecalculateAll(minSamples int) ([]*flywheel.RecalculationResult, error)
	RecalculateCompany(companyID uint, minSamples int) (*flywheel.RecalculationResult, error)
	Rollback(companyID uint) error
	GetHistory(companyID uint) ([]*models.CompanyProfileUpdateHistory, error)
}
