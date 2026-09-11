package repositories

import (
	"strings"

	"Backend/internal/models"

	"gorm.io/gorm"
)

// CompanyPublicRepository は CompanyRepository の名前・ID 参照に
// #1203 の可視性ガードを掛けたラッパ。
//
// CompanyRepository は管理画面用でフィルタを持たないため、
// 学生・無認証向けのサービス（企業実在確認 / 面接 / 履歴書）が直接使うと
// 審査前のゲスト投稿企業がそのまま返る。
// /companies 系は CompanyQueryRepository で塞いだが、
// /companies/web-search と /companies/validate、および面接・履歴書の
// 企業ブリーフはこちらを経由する。
//
// 埋め込みにより GetWeightProfile など残りの参照系はそのまま引き継ぐ。
type CompanyPublicRepository struct {
	*CompanyRepository
}

func NewCompanyPublicRepository(db *gorm.DB) *CompanyPublicRepository {
	return &CompanyPublicRepository{CompanyRepository: NewCompanyRepository(db)}
}

// FindByID IDで企業を取得（審査前のゲスト投稿は返さない）
func (r *CompanyPublicRepository) FindByID(id uint) (*models.Company, error) {
	var company models.Company
	err := r.db.
		Where("id = ?", id).
		Where(guestEntryVisibilityGuard("companies.id")).
		First(&company).Error
	if err != nil {
		return nil, err
	}
	return &company, nil
}

// FindByName 企業名で取得（審査前のゲスト投稿は返さない）
func (r *CompanyPublicRepository) FindByName(name string) (*models.Company, error) {
	var company models.Company
	err := r.db.
		Where("name = ?", name).
		Where(guestEntryVisibilityGuard("companies.id")).
		First(&company).Error
	if err != nil {
		return nil, err
	}
	return &company, nil
}

// FindAllActiveNames アクティブ企業のIDと名前を取得（審査前のゲスト投稿は返さない）
func (r *CompanyPublicRepository) FindAllActiveNames(q string) ([]models.CompanyName, error) {
	var names []models.CompanyName
	query := r.db.Model(&models.Company{}).
		Select("id, name").
		Where("is_active = ?", true).
		Where(guestEntryVisibilityGuard("companies.id"))
	if q = strings.TrimSpace(q); q != "" {
		query = query.Where("name LIKE ?", "%"+q+"%")
	}
	err := query.Order("name asc").Find(&names).Error
	return names, err
}
