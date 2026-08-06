package interfaces

import (
	"Backend/internal/models"
	"Backend/internal/services/admin"
)

// ScraperSessionService スクレイパーセッションサービスのインターフェース
type ScraperSessionService interface {
	List() ([]models.ScraperSession, error)
	Upsert(payload admin.ScraperSessionPayload) (*models.ScraperSession, error)
	Delete(siteKey string) error
}
