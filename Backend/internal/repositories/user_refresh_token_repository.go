package repositories

import (
	"time"

	"gorm.io/gorm"

	"Backend/internal/models"
)

// UserRefreshTokenRepository はリフレッシュトークンの永続化を担当する (#616)
type UserRefreshTokenRepository struct {
	db *gorm.DB
}

func NewUserRefreshTokenRepository(db *gorm.DB) *UserRefreshTokenRepository {
	return &UserRefreshTokenRepository{db: db}
}

// Create はリフレッシュトークンを保存する
func (r *UserRefreshTokenRepository) Create(token *models.UserRefreshToken) error {
	return r.db.Create(token).Error
}

// FindByHash はハッシュからトークンを取得する（見つからない場合は nil, nil）
func (r *UserRefreshTokenRepository) FindByHash(hash string) (*models.UserRefreshToken, error) {
	var m models.UserRefreshToken
	err := r.db.Where("token_hash = ?", hash).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Revoke はトークンを失効させる（既に失効済みの場合は上書きしない）
func (r *UserRefreshTokenRepository) Revoke(id uint, at time.Time) error {
	return r.db.Model(&models.UserRefreshToken{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", at).Error
}

// RevokeAllByUser はユーザーの有効なトークンをすべて失効させる
// （ログアウト全端末・パスワード変更・アカウント停止時に使用）
func (r *UserRefreshTokenRepository) RevokeAllByUser(userID uint, at time.Time) error {
	return r.db.Model(&models.UserRefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", at).Error
}

// DeleteExpired は期限切れトークンを物理削除する（定期クリーンアップ用）
func (r *UserRefreshTokenRepository) DeleteExpired(before time.Time) error {
	return r.db.Where("expires_at < ?", before).
		Delete(&models.UserRefreshToken{}).Error
}
