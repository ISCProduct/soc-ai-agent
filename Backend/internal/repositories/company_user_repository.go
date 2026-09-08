package repositories

import (
	"Backend/internal/models"
	"strings"
	"time"

	"gorm.io/gorm"
)

type CompanyUserRepository struct {
	db *gorm.DB
}

func NewCompanyUserRepository(db *gorm.DB) *CompanyUserRepository {
	return &CompanyUserRepository{db: db}
}

func (r *CompanyUserRepository) Create(user *models.CompanyUser) error {
	return r.db.Create(user).Error
}

func (r *CompanyUserRepository) Update(user *models.CompanyUser) error {
	return r.db.Save(user).Error
}

func (r *CompanyUserRepository) FindByID(id uint) (*models.CompanyUser, error) {
	var m models.CompanyUser
	err := r.db.First(&m, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *CompanyUserRepository) FindByEmail(email string) (*models.CompanyUser, error) {
	var m models.CompanyUser
	err := r.db.Where("email = ?", strings.ToLower(strings.TrimSpace(email))).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// FindByInviteTokenHash は招待トークンの SHA-256 hex で検索する。
// 平文トークンはDBに保存しない（#1196）。
func (r *CompanyUserRepository) FindByInviteTokenHash(hash string) (*models.CompanyUser, error) {
	var m models.CompanyUser
	err := r.db.Where("invite_token_hash = ?", hash).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// FindByPasswordResetTokenHash はパスワードリセットトークンの SHA-256 hex で検索する。
func (r *CompanyUserRepository) FindByPasswordResetTokenHash(hash string) (*models.CompanyUser, error) {
	var m models.CompanyUser
	err := r.db.Where("password_reset_token_hash = ?", hash).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// SetPasswordResetToken はリセットトークンの列だけを更新する（#1196）。
//
// Save(全カラム更新)を使うと read-modify-write の窓で他の更新を巻き戻す。
// 特に disabled_at を消してしまうと、無効化したはずのアカウントが復活する。
func (r *CompanyUserRepository) SetPasswordResetToken(id uint, hash *string, expiresAt *time.Time) error {
	return r.db.Model(&models.CompanyUser{}).Where("id = ?", id).
		Updates(map[string]any{
			"password_reset_token_hash": hash,
			"password_reset_expires_at": expiresAt,
		}).Error
}

// ApplyPasswordReset はパスワードを設定し、同時にリセットトークンを使い捨てにする。
//
// disabled_at IS NULL を条件に含めるのは、bcrypt(cost 12, 約300ms)の間に
// 管理者が無効化した場合に、無効なアカウントのパスワードだけ書き換わるのを防ぐため。
// 更新行数が0なら呼び出し側で失敗として扱う。
func (r *CompanyUserRepository) ApplyPasswordReset(id uint, hashedPassword string) (int64, error) {
	res := r.db.Model(&models.CompanyUser{}).
		Where("id = ? AND disabled_at IS NULL", id).
		Updates(map[string]any{
			"password":                  hashedPassword,
			"password_reset_token_hash": nil,
			"password_reset_expires_at": nil,
		})
	return res.RowsAffected, res.Error
}

// SetDisabled は無効化状態だけを更新する。
// 無効化時はリセット導線も同時に塞ぐ（無効化の迂回を防ぐ）。
func (r *CompanyUserRepository) SetDisabled(id uint, disabledAt *time.Time) error {
	values := map[string]any{"disabled_at": disabledAt}
	if disabledAt != nil {
		values["password_reset_token_hash"] = nil
		values["password_reset_expires_at"] = nil
	}
	return r.db.Model(&models.CompanyUser{}).Where("id = ?", id).Updates(values).Error
}

func (r *CompanyUserRepository) ListByCompanyID(companyID uint) ([]models.CompanyUser, error) {
	var users []models.CompanyUser
	err := r.db.Where("company_id = ?", companyID).Order("id ASC").Find(&users).Error
	return users, err
}
