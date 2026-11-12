package repositories

import (
	"fmt"
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

// DeleteExpired は expires_at が before より前のトークンを、期限の古い順に最大 limit 件だけ物理削除し、
// 削除した件数を返す（定期クリーンアップ用、#2153）。
//
// 1文で消す件数を limit で抑え、行ロックを持つ時間を短くする。残りは呼び出し側が次のバッチで消す。
// expires_at にはインデックス（idx_user_refresh_tokens_expires_at）があるので、
// ロックが掛かるのは消す範囲の行だけで済む。
func (r *UserRefreshTokenRepository) DeleteExpired(before time.Time, limit int) (int64, error) {
	// GORM は Limit(-1) を「上限なし」と解釈するので、0 以下はここで弾く。
	if limit <= 0 {
		return 0, fmt.Errorf("limit は 1 以上を指定してください: %d", limit)
	}
	res := r.db.Where("expires_at < ?", before).
		Order("expires_at").
		Limit(limit).
		Delete(&models.UserRefreshToken{})
	return res.RowsAffected, res.Error
}
