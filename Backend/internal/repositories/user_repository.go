package repositories

import (
	"Backend/internal/models"
	"errors"
	"time"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// CreateUser 新規ユーザー作成（デフォルト組織へ所属）
func (r *UserRepository) CreateUser(user *models.User) error {
	m := new(models.User)
	*m = *user
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
		*user = *m
		return nil
	})
}

// GetUserByEmail メールアドレスでユーザー取得
func (r *UserRepository) GetUserByEmail(email string) (*models.User, error) {
	var m models.User
	if err := r.db.Where("email = ?", email).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// GetUserByID IDでユーザー取得
func (r *UserRepository) GetUserByID(id uint) (*models.User, error) {
	var m models.User
	if err := r.db.First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// scopeUsersBySchool は「担当校の在籍者」の絞り込みを1箇所にまとめる（#1675）。
//
// 学校だけで絞ると他学園の学生が混ざる。users.school_id はプロフィールの学校名
// （本人の自己申告）から解決されるため、学校名を他学園の校名に書き替えた学生が
// その学校の一覧に現れてしまう。解決側（SchoolRepository.FindByName）は
// organization_id で絞り、既存の汚染行は migration 000054 で外したが、
// 境界の判定はデータの綺麗さに依存させない。学校の学園と本人の学園の一致を
// クエリ自体で要求する。
//
// 担当校スコープ（EchoAdminSchoolScope）の結果を受けて users を絞る経路は
// すべてここを通す。1つでも素の school_id で絞ると、同じ穴がその経路に残る。
// users を JOIN している呼び出し元でも使えるよう列は users. で修飾する。
func scopeUsersBySchool(q *gorm.DB, schoolID *uint) *gorm.DB {
	if schoolID == nil {
		return q
	}
	return q.Where("users.school_id = ?", *schoolID).
		Where("users.organization_id = (SELECT organization_id FROM schools WHERE id = ?)", *schoolID)
}

// ListUsersPaged ページング付きユーザー一覧を取得
// schoolID が非nilの場合は個別校で絞り込む。
func (r *UserRepository) ListUsersPaged(limit, offset int, query string, schoolID *uint) ([]models.User, int64, error) {
	var ms []models.User
	var total int64

	q := r.db.Model(&models.User{}).Where("withdrawn_at IS NULL")
	if query != "" {
		like := "%" + query + "%"
		q = q.Where("name LIKE ? OR email LIKE ? OR school_name LIKE ?", like, like, like)
	}
	q = scopeUsersBySchool(q, schoolID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at desc").Limit(limit).Offset(offset).Find(&ms).Error
	if err != nil {
		return nil, 0, err
	}
	return ms, total, nil
}

// UpdateUser ユーザー情報更新
func (r *UserRepository) UpdateUser(user *models.User) error {
	m := *user
	return r.db.Save(&m).Error
}

// DeleteUser ユーザー削除
func (r *UserRepository) DeleteUser(id uint) error {
	return r.db.Delete(&models.User{}, id).Error
}

// GetUserByVerificationToken メール認証トークンでユーザー取得
func (r *UserRepository) GetUserByVerificationToken(token string) (*models.User, error) {
	var m models.User
	if err := r.db.Where("email_verification_token = ?", token).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// RenewEmailVerification はメール認証トークンと期限だけを、未認証の行に限って更新する。
// 行を丸ごと保存(UpdateUser)しないので、他の処理が同時に書いた列(認証済みの印など)を巻き戻さない。
func (r *UserRepository) RenewEmailVerification(userID uint, token string, expires time.Time) (bool, error) {
	res := r.db.Model(&models.User{}).
		Where("id = ? AND email_verified_at IS NULL", userID).
		Updates(map[string]any{
			"email_verification_token":   token,
			"email_verification_expires": expires,
		})
	return res.RowsAffected > 0, res.Error
}

// GetUserByPasswordResetToken パスワードリセットトークンでユーザー取得
func (r *UserRepository) GetUserByPasswordResetToken(token string) (*models.User, error) {
	var m models.User
	if err := r.db.Where("password_reset_token = ?", token).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// GetUserByOAuth OAuth情報でユーザー取得
func (r *UserRepository) GetUserByOAuth(provider, oauthID string) (*models.User, error) {
	var m models.User
	if err := r.db.Where("oauth_provider = ? AND oauth_id = ?", provider, oauthID).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// ListStudentsPaged は生徒のみをページング付きで返す（#1027）。
//
// ListUsersPaged は role / is_guest / is_admin で絞っていないため、
// 管理者やゲストも混ざる。教員向けの生徒一覧ではそれらを除く。
// schoolID が非nilなら担当校で絞る（EchoAdminSchoolScope の結果を渡す）。
func (r *UserRepository) ListStudentsPaged(limit, offset int, query string, schoolID *uint) ([]models.User, int64, error) {
	var ms []models.User
	var total int64

	q := r.db.Model(&models.User{}).
		Where("withdrawn_at IS NULL").
		Where("role = ?", "student").
		Where("is_guest = ?", false).
		Where("is_admin = ?", false)
	if query != "" {
		like := "%" + query + "%"
		q = q.Where("name LIKE ? OR email LIKE ? OR school_name LIKE ?", like, like, like)
	}
	q = scopeUsersBySchool(q, schoolID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	// id を第2キーにして、同一時刻でもページ境界がずれないようにする。
	if err := q.Order("created_at desc, id desc").Limit(limit).Offset(offset).Find(&ms).Error; err != nil {
		return nil, 0, err
	}
	return ms, total, nil
}
