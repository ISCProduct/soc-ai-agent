package repositories

import (
	"Backend/internal/models"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type PendingRegistrationRepository struct {
	db *gorm.DB
}

func NewPendingRegistrationRepository(db *gorm.DB) *PendingRegistrationRepository {
	return &PendingRegistrationRepository{db: db}
}

func (r *PendingRegistrationRepository) Create(p *models.PendingRegistration) error {
	m := p
	return r.db.Create(m).Error
}

func (r *PendingRegistrationRepository) FindByToken(token string) (*models.PendingRegistration, error) {
	var m models.PendingRegistration
	err := r.db.Where("token = ? AND expires_at > ?", token, time.Now()).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *PendingRegistrationRepository) DeleteByEmail(email string) error {
	return r.db.Where("email = ?", email).Delete(&models.PendingRegistration{}).Error
}

// DeleteExpired は期限切れ（expires_at <= now）の仮登録を、期限の古い順に最大 limit 件だけ物理削除し、
// 削除した件数を返す（定期クリーンアップ用、#2153）。
//
// 期限ちょうどの行も消す。FindByToken が expires_at > now だけを有効とするので、その時点で使えない。
// 1文で消す件数を limit で抑え、行ロックを持つ時間を短くする。残りは呼び出し側が次のバッチで消す。
// expires_at のインデックス（migrations/000067）で、ロックが掛かるのは消す範囲の行だけで済む。
// 消す行はトークンで指定せず条件で選ぶので、GORM が SQL をログに出してもトークンは出ない。
// 複数台で同時に走っても、後の文は先の文がロックを放すのを待ってから残りの行を消すだけで、壊れない。
func (r *PendingRegistrationRepository) DeleteExpired(now time.Time, limit int) (int64, error) {
	// GORM は Limit(-1) を「上限なし」と解釈するので、0 以下はここで弾く。
	// 素通しすると、上限を付けたつもりで全件を1文で消してしまう。
	if limit <= 0 {
		return 0, fmt.Errorf("limit は 1 以上を指定してください: %d", limit)
	}
	res := r.db.Where("expires_at <= ?", now).
		Order("expires_at").
		Limit(limit).
		Delete(&models.PendingRegistration{})
	return res.RowsAffected, res.Error
}
