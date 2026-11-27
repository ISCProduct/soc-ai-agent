package repositories

import (
	"Backend/internal/models"
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CompanyBusinessTagRepository は企業の事業タグ（SOCAIAGENT-717）を扱う。
type CompanyBusinessTagRepository struct {
	db *gorm.DB
}

func NewCompanyBusinessTagRepository(db *gorm.DB) *CompanyBusinessTagRepository {
	return &CompanyBusinessTagRepository{db: db}
}

// ListAll は共通のタグ一覧を名前順で返す。
func (r *CompanyBusinessTagRepository) ListAll(ctx context.Context) ([]models.BusinessTag, error) {
	tags := []models.BusinessTag{}
	err := r.db.WithContext(ctx).Order("name ASC").Find(&tags).Error
	return tags, err
}

// NamesByCompanyIDs は企業ごとのタグ名を名前順で返す（相関図の N+1 回避）。
func (r *CompanyBusinessTagRepository) NamesByCompanyIDs(ctx context.Context, companyIDs []uint) (map[uint][]string, error) {
	out := map[uint][]string{}
	if len(companyIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		CompanyID uint
		Name      string
	}
	err := r.db.WithContext(ctx).
		Table("company_business_tags AS cbt").
		Select("cbt.company_id, bt.name").
		Joins("JOIN business_tags AS bt ON bt.id = cbt.business_tag_id").
		Where("cbt.company_id IN ?", companyIDs).
		Order("cbt.company_id ASC, bt.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.CompanyID] = append(out[row.CompanyID], row.Name)
	}
	return out, nil
}

// ReplaceForCompany は企業のタグを names で丸ごと置き換える。一覧に無い名前は作る。
func (r *CompanyBusinessTagRepository) ReplaceForCompany(ctx context.Context, companyID uint, names []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("company_id = ?", companyID).Delete(&models.CompanyBusinessTag{}).Error; err != nil {
			return err
		}
		if len(names) == 0 {
			return nil
		}
		newTags := make([]models.BusinessTag, len(names))
		for i, name := range names {
			newTags[i] = models.BusinessTag{Name: name}
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&newTags).Error; err != nil {
			return err
		}
		var tags []models.BusinessTag
		if err := tx.Where("name IN ?", names).Find(&tags).Error; err != nil {
			return err
		}
		links := make([]models.CompanyBusinessTag, len(tags))
		for i, tag := range tags {
			links[i] = models.CompanyBusinessTag{CompanyID: companyID, BusinessTagID: tag.ID}
		}
		return tx.Create(&links).Error
	})
}
