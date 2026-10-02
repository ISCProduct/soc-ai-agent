package repositories

import (
	"time"

	"Backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ScoutRepository はスカウト送信・テンプレート・ブロックを扱う（#1095）。
// 企業向け操作は company_id、学生向け操作は user_id でスコープする。
type ScoutRepository struct {
	db *gorm.DB
}

func NewScoutRepository(db *gorm.DB) *ScoutRepository {
	return &ScoutRepository{db: db}
}

func (r *ScoutRepository) CreateTemplate(t *models.ScoutTemplate) error {
	return r.db.Create(t).Error
}

func (r *ScoutRepository) UpdateTemplate(companyID, id uint, title, body string) error {
	res := r.db.Model(&models.ScoutTemplate{}).
		Where("id = ? AND company_id = ?", id, companyID).
		Updates(map[string]any{"title": title, "body": body})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *ScoutRepository) DeleteTemplate(companyID, id uint) error {
	res := r.db.Where("id = ? AND company_id = ?", id, companyID).
		Delete(&models.ScoutTemplate{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *ScoutRepository) ListTemplates(companyID uint) ([]models.ScoutTemplate, error) {
	items := []models.ScoutTemplate{}
	err := r.db.Where("company_id = ?", companyID).
		Order("id DESC").Find(&items).Error
	return items, err
}

func (r *ScoutRepository) FindTemplate(companyID, id uint) (*models.ScoutTemplate, error) {
	var t models.ScoutTemplate
	err := r.db.Where("id = ? AND company_id = ?", id, companyID).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *ScoutRepository) CreateScout(s *models.Scout) error {
	return r.db.Create(s).Error
}

func (r *ScoutRepository) ListByCompany(companyID uint, limit, offset int) ([]models.Scout, int64, error) {
	var total int64
	q := r.db.Model(&models.Scout{}).Where("company_id = ?", companyID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := []models.Scout{}
	err := q.Order("id DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (r *ScoutRepository) ListByUser(userID uint, limit, offset int) ([]models.Scout, int64, error) {
	var total int64
	q := r.db.Model(&models.Scout{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := []models.Scout{}
	err := q.Order("id DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (r *ScoutRepository) FindByIDForUser(userID, id uint) (*models.Scout, error) {
	var s models.Scout
	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ScoutRepository) UpdateStatusForUser(userID, id uint, fromStatuses []string, toStatus string) error {
	res := r.db.Model(&models.Scout{}).
		Where("id = ? AND user_id = ? AND status IN ?", id, userID, fromStatuses).
		Update("status", toStatus)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// LatestBetween は同一企業→同一学生の直近スカウトを返す（クールダウン判定用）。
func (r *ScoutRepository) LatestBetween(companyID, userID uint) (*models.Scout, error) {
	var s models.Scout
	err := r.db.Where("company_id = ? AND user_id = ?", companyID, userID).
		Order("created_at DESC").First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ScoutRepository) IsBlocked(userID, companyID uint) (bool, error) {
	var n int64
	err := r.db.Model(&models.ScoutCompanyBlock{}).
		Where("user_id = ? AND company_id = ?", userID, companyID).
		Count(&n).Error
	return n > 0, err
}

func (r *ScoutRepository) BlockCompany(userID, companyID uint) error {
	b := models.ScoutCompanyBlock{
		UserID:    userID,
		CompanyID: companyID,
		CreatedAt: time.Now(),
	}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&b).Error
}

func (r *ScoutRepository) ListBlockedCompanyIDs(userID uint) ([]uint, error) {
	ids := []uint{}
	err := r.db.Model(&models.ScoutCompanyBlock{}).
		Where("user_id = ?", userID).
		Pluck("company_id", &ids).Error
	return ids, err
}
