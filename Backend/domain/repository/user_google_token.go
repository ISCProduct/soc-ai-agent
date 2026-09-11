package repository

import "Backend/internal/models"

// UserGoogleTokenRepository はGoogleカレンダー連携トークンの永続化インターフェース。
type UserGoogleTokenRepository interface {
	FindByUserID(userID uint) (*models.UserGoogleToken, error)
	Upsert(token *models.UserGoogleToken) error
	DeleteByUserID(userID uint) error
}
