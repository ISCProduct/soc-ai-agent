package repositories

import (
	"fmt"
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

// Delete はトークンを完全に削除する。明示的なログアウトで使用し、
// Revoke のローテーション猶予期間（並行リフレッシュ対策）を適用させない。
func (r *CompanyUserRefreshTokenRepository) Delete(id uint) error {
	return r.db.Delete(&models.CompanyUserRefreshToken{}, id).Error
}

// DeleteExpired は expires_at が before より前の企業ユーザーのリフレッシュトークンを、期限の古い順に
// 最大 limit 件だけ物理削除し、削除した件数を返す（定期クリーンアップ用、#2184）。
// UserRefreshTokenRepository.DeleteExpired と同じ形。expires_at には索引
// （idx_company_user_refresh_tokens_expires、migrations/000017）があるので、ロックは消す範囲の行に限られる。
func (r *CompanyUserRefreshTokenRepository) DeleteExpired(before time.Time, limit int) (int64, error) {
	// GORM は Limit(-1) を「上限なし」と解釈するので、0 以下はここで弾く。
	if limit <= 0 {
		return 0, fmt.Errorf("limit は 1 以上を指定してください: %d", limit)
	}
	res := r.db.Where("expires_at < ?", before).
		Order("expires_at").
		Limit(limit).
		Delete(&models.CompanyUserRefreshToken{})
	return res.RowsAffected, res.Error
}
