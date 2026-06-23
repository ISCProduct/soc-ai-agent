package repositories

import (
	"Backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserGoogleTokenRepository struct {
	db *gorm.DB
}

func NewUserGoogleTokenRepository(db *gorm.DB) *UserGoogleTokenRepository {
	return &UserGoogleTokenRepository{db: db}
}

func (r *UserGoogleTokenRepository) FindByUserID(userID uint) (*models.UserGoogleToken, error) {
	var token models.UserGoogleToken
	err := r.db.Where("user_id = ?", userID).First(&token).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

// Upsert はUserIDをキーにして存在すれば更新、なければ挿入する。
func (r *UserGoogleTokenRepository) Upsert(token *models.UserGoogleToken) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"encrypted_access_token", "encrypted_refresh_token", "expiry", "calendar_id", "updated_at"}),
	}).Create(token).Error
}

func (r *UserGoogleTokenRepository) DeleteByUserID(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&models.UserGoogleToken{}).Error
}
