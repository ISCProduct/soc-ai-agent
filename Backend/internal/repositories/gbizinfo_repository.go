package repositories

import (
	"Backend/internal/models"
	"errors"

	"gorm.io/gorm"
)

type GBizInfoRepository struct {
	db *gorm.DB
}

func NewGBizInfoRepository(db *gorm.DB) *GBizInfoRepository {
	return &GBizInfoRepository{db: db}
}

func (r *GBizInfoRepository) UpsertProfile(profile *models.GBizCompanyProfile) error {
	var existing models.GBizCompanyProfile
	err := r.db.Where("company_id = ?", profile.CompanyID).First(&existing).Error
	if err == nil {
		profile.ID = existing.ID
		return r.db.Save(profile).Error
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.db.Create(profile).Error
}

func (r *GBizInfoRepository) ReplaceProcurements(companyID uint, rows []models.GBizProcurement) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("company_id = ?", companyID).Delete(&models.GBizProcurement{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func (r *GBizInfoRepository) ReplaceSubsidies(companyID uint, rows []models.GBizSubsidy) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("company_id = ?", companyID).Delete(&models.GBizSubsidy{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func (r *GBizInfoRepository) ReplaceFinances(companyID uint, rows []models.GBizFinance) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("company_id = ?", companyID).Delete(&models.GBizFinance{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func (r *GBizInfoRepository) UpsertWorkplace(workplace *models.GBizWorkplace) error {
	var existing models.GBizWorkplace
	err := r.db.Where("company_id = ?", workplace.CompanyID).First(&existing).Error
	if err == nil {
		workplace.ID = existing.ID
		return r.db.Save(workplace).Error
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.db.Create(workplace).Error
}
