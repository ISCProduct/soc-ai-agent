package repositories

import (
	"Backend/internal/models"

	"gorm.io/gorm"
)

type ScraperSessionRepository struct {
	db *gorm.DB
}

func NewScraperSessionRepository(db *gorm.DB) *ScraperSessionRepository {
	return &ScraperSessionRepository{db: db}
}

func (r *ScraperSessionRepository) List() ([]models.ScraperSession, error) {
	var sessions []models.ScraperSession
	err := r.db.Order("site_key asc").Find(&sessions).Error
	return sessions, err
}

func (r *ScraperSessionRepository) GetBySiteKey(siteKey string) (*models.ScraperSession, error) {
	var session models.ScraperSession
	err := r.db.Where("site_key = ?", siteKey).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *ScraperSessionRepository) Upsert(session *models.ScraperSession) error {
	var existing models.ScraperSession
	err := r.db.Where("site_key = ?", session.SiteKey).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(session).Error
	}
	if err != nil {
		return err
	}
	session.ID = existing.ID
	return r.db.Save(session).Error
}

func (r *ScraperSessionRepository) DeleteBySiteKey(siteKey string) error {
	return r.db.Where("site_key = ?", siteKey).Delete(&models.ScraperSession{}).Error
}
