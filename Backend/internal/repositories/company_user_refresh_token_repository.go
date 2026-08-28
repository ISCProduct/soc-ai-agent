package repositories

import (
	"time"

	"Backend/internal/models"

	"gorm.io/gorm"
)

type CompanyUserRefreshTokenRepository struct {
	db *gorm.DB
}

func NewCompanyUserRefreshTokenRepository(db *gorm.DB) *CompanyUserRefreshTokenRepository {
	return &CompanyUserRefreshTokenRepository{db: db}
}

func (r *CompanyUserRefreshTokenRepository) Create(token *models.CompanyUserRefreshToken) error {
	return r.db.Create(token).Error
}

func (r *CompanyUserRefreshTokenRepository) FindByHash(hash string) (*models.CompanyUserRefreshToken, error) {
	var m models.CompanyUserRefreshToken
	err := r.db.Where("token_hash = ?", hash).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *CompanyUserRefreshTokenRepository) Revoke(id uint, at time.Time) error {
	return r.db.Model(&models.CompanyUserRefreshToken{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", at).Error
}

func (r *CompanyUserRefreshTokenRepository) RevokeAllByCompanyUser(companyUserID uint, at time.Time) error {
	return r.db.Model(&models.CompanyUserRefreshToken{}).
		Where("company_user_id = ? AND revoked_at IS NULL", companyUserID).
		Update("revoked_at", at).Error
}
