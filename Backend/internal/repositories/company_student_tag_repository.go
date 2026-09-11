package repositories

import (
	"Backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CompanyStudentTagRepository は企業ごとの学生タグを扱う（#1094）。
// 全メソッドが company_id を必須引数に取り、他社データへ越境しない。
type CompanyStudentTagRepository struct {
	db *gorm.DB
}

func NewCompanyStudentTagRepository(db *gorm.DB) *CompanyStudentTagRepository {
	return &CompanyStudentTagRepository{db: db}
}

// Add はタグを付与する。同一(company,user,tag)が既にあれば何もしない。
func (r *CompanyStudentTagRepository) Add(tag *models.CompanyStudentTag) error {
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(tag).Error
}

// Delete は自社のタグのみ削除する。
func (r *CompanyStudentTagRepository) Delete(companyID, tagID uint) error {
	return r.db.Where("id = ? AND company_id = ?", tagID, companyID).
		Delete(&models.CompanyStudentTag{}).Error
}

// ListByUser は指定学生に自社が付けたタグを返す。
func (r *CompanyStudentTagRepository) ListByUser(companyID, userID uint) ([]models.CompanyStudentTag, error) {
	tags := []models.CompanyStudentTag{}
	err := r.db.Where("company_id = ? AND user_id = ?", companyID, userID).
		Order("id ASC").Find(&tags).Error
	return tags, err
}

// ListByUsers は複数学生分のタグをまとめて取得する（一覧のN+1回避）。
func (r *CompanyStudentTagRepository) ListByUsers(companyID uint, userIDs []uint) (map[uint][]models.CompanyStudentTag, error) {
	out := map[uint][]models.CompanyStudentTag{}
	if len(userIDs) == 0 {
		return out, nil
	}
	tags := []models.CompanyStudentTag{}
	err := r.db.Where("company_id = ? AND user_id IN ?", companyID, userIDs).
		Order("id ASC").Find(&tags).Error
	if err != nil {
		return nil, err
	}
	for _, t := range tags {
		out[t.UserID] = append(out[t.UserID], t)
	}
	return out, nil
}

// ListTagNames は自社が使用中のタグ名を重複なく返す（タグ候補の提示用）。
func (r *CompanyStudentTagRepository) ListTagNames(companyID uint) ([]string, error) {
	names := []string{}
	err := r.db.Model(&models.CompanyStudentTag{}).
		Where("company_id = ?", companyID).
		Distinct().Order("tag_name ASC").Pluck("tag_name", &names).Error
	return names, err
}
