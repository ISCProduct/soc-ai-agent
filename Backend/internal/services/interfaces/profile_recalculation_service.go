package interfaces

import (
	"Backend/internal/models"
	"Backend/internal/services"
)

// ProfileRecalculationService プロファイル再計算サービスのインターフェース
type ProfileRecalculationService interface {
	RecalculateAll(minSamples int) ([]*services.RecalculationResult, error)
	RecalculateCompany(companyID uint, minSamples int) (*services.RecalculationResult, error)
	Rollback(companyID uint) error
	GetHistory(companyID uint) ([]*models.CompanyProfileUpdateHistory, error)
}
