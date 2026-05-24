package interfaces

import (
	"Backend/internal/models"
	"Backend/internal/services"
)

// CrawlService クロールサービスのインターフェース
type CrawlService interface {
	ListSources() ([]models.CrawlSource, error)
	ListRuns(sourceID uint, limit int) ([]models.CrawlRun, error)
	CreateSource(payload services.CrawlSourcePayload) (*models.CrawlSource, error)
	UpdateSource(id uint, payload services.CrawlSourcePayload) (*models.CrawlSource, error)
	RunSource(id uint) (*models.CrawlRun, error)
}
