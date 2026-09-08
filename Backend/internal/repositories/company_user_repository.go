package repositories

import (
	"Backend/internal/models"
	"strings"

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

func (r *CompanyUserRepository) ListByCompanyID(companyID uint) ([]models.CompanyUser, error) {
	var users []models.CompanyUser
	err := r.db.Where("company_id = ?", companyID).Order("id ASC").Find(&users).Error
	return users, err
}
