package repositories

import (
	"Backend/domain/entity"
	"Backend/domain/mapper"
	"Backend/internal/models"
	"errors"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// CreateUser 新規ユーザー作成（デフォルト組織へ所属）
func (r *UserRepository) CreateUser(user *entity.User) error {
	m := mapper.UserFromEntity(user)
	if m.OrganizationID == 0 {
		m.OrganizationID = models.DefaultOrganizationID
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		role := models.OrgRoleMember
		if m.IsAdmin {
			role = models.OrgRoleOwner
		}
		membership := models.OrganizationMembership{
			OrganizationID: m.OrganizationID,
			UserID:         m.ID,
			Role:           role,
		}
		if err := tx.Create(&membership).Error; err != nil {
			return err
		}
		*user = *mapper.UserToEntity(m)
		return nil
	})
}

// GetUserByEmail メールアドレスでユーザー取得
func (r *UserRepository) GetUserByEmail(email string) (*entity.User, error) {
	var m models.User
	if err := r.db.Where("email = ?", email).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return mapper.UserToEntity(&m), nil
}

// GetUserByID IDでユーザー取得
func (r *UserRepository) GetUserByID(id uint) (*entity.User, error) {
	var m models.User
	if err := r.db.First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return mapper.UserToEntity(&m), nil
}

// ListUsers ユーザー一覧を取得
func (r *UserRepository) ListUsers() ([]entity.User, error) {
	var ms []models.User
	err := r.db.Order("created_at desc").Find(&ms).Error
	if err != nil {
		return nil, err
	}
	result := make([]entity.User, len(ms))
	for i, m := range ms {
		e := mapper.UserToEntity(&m)
		result[i] = *e
	}
	return result, nil
}

// ListUsersPaged ページング付きユーザー一覧を取得
func (r *UserRepository) ListUsersPaged(limit, offset int, query string) ([]entity.User, int64, error) {
	var ms []models.User
	var total int64

	q := r.db.Model(&models.User{}).Where("withdrawn_at IS NULL")
	if query != "" {
		like := "%" + query + "%"
		q = q.Where("name LIKE ? OR email LIKE ? OR school_name LIKE ?", like, like, like)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at desc").Limit(limit).Offset(offset).Find(&ms).Error
	if err != nil {
		return nil, 0, err
	}
	result := make([]entity.User, len(ms))
	for i, m := range ms {
		e := mapper.UserToEntity(&m)
		result[i] = *e
	}
	return result, total, nil
}

// UpdateUser ユーザー情報更新
func (r *UserRepository) UpdateUser(user *entity.User) error {
	m := mapper.UserFromEntity(user)
	return r.db.Save(m).Error
}

// DeleteUser ユーザー削除
func (r *UserRepository) DeleteUser(id uint) error {
	return r.db.Delete(&models.User{}, id).Error
}

// GetUserByVerificationToken メール認証トークンでユーザー取得
func (r *UserRepository) GetUserByVerificationToken(token string) (*entity.User, error) {
	var m models.User
	if err := r.db.Where("email_verification_token = ?", token).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return mapper.UserToEntity(&m), nil
}

// GetUserByPasswordResetToken パスワードリセットトークンでユーザー取得
func (r *UserRepository) GetUserByPasswordResetToken(token string) (*entity.User, error) {
	var m models.User
	if err := r.db.Where("password_reset_token = ?", token).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return mapper.UserToEntity(&m), nil
}

// GetUserByOAuth OAuth情報でユーザー取得
func (r *UserRepository) GetUserByOAuth(provider, oauthID string) (*entity.User, error) {
	var m models.User
	if err := r.db.Where("oauth_provider = ? AND oauth_id = ?", provider, oauthID).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return mapper.UserToEntity(&m), nil
}
