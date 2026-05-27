package interfaces

import (
	"Backend/internal/models"
	"Backend/internal/services"
)

// ScraperSessionService スクレイパーセッションサービスのインターフェース
type ScraperSessionService interface {
	List() ([]models.ScraperSession, error)
	Upsert(payload services.ScraperSessionPayload) (*models.ScraperSession, error)
	Delete(siteKey string) error
}
