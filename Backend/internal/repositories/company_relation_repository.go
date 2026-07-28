package repositories

import (
	"Backend/internal/models"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type CompanyRelationRepository struct {
	db *gorm.DB
}

func NewCompanyRelationRepository(db *gorm.DB) *CompanyRelationRepository {
	return &CompanyRelationRepository{db: db}
}

func (r *CompanyRelationRepository) UpsertBusinessRelation(fromID, toID uint, relationType, description string) error {
	var existing models.CompanyRelation
	err := r.db.Where("from_id = ? AND to_id = ? AND relation_type = ?", fromID, toID, relationType).
		First(&existing).Error
	if err == nil {
		existing.Description = description
		existing.IsActive = true
		return r.db.Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	relation := models.CompanyRelation{
		FromID:       &fromID,
		ToID:         &toID,
		RelationType: relationType,
		Description:  description,
		IsActive:     true,
	}
	return r.db.Create(&relation).Error
}

// UpsertCapitalRelation は資本関係を parent_id / child_id で保存する。
// 資本図（CompanyDiagram）はこのカラムを参照するため、business の from/to には書かない。
func (r *CompanyRelationRepository) UpsertCapitalRelation(parentID, childID uint, relationType string, ratio *float64, description string) error {
	if !models.IsCapitalRelationType(relationType) {
		relationType = "capital_affiliate"
	}

	var existing models.CompanyRelation
	err := r.db.Where("parent_id = ? AND child_id = ? AND relation_type = ?", parentID, childID, relationType).
		First(&existing).Error
	if err == nil {
		existing.Description = description
		existing.Ratio = ratio
		existing.IsActive = true
		return r.db.Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	relation := models.CompanyRelation{
		ParentID:     &parentID,
		ChildID:      &childID,
		RelationType: relationType,
		Ratio:        ratio,
		Description:  description,
		IsActive:     true,
	}
	return r.db.Create(&relation).Error
}

// GetRelationsByCompanyID は指定企業に関連する関係を返す。
func (r *CompanyRelationRepository) GetRelationsByCompanyID(companyID uint) ([]models.CompanyRelation, error) {
	var relations []models.CompanyRelation
	err := r.db.
		Preload("Parent").
		Preload("Child").
		Preload("From").
		Preload("To").
		Where("parent_id = ? OR child_id = ? OR from_id = ? OR to_id = ?",
			companyID, companyID, companyID, companyID).
		Where("is_active = ?", true).
		Find(&relations).Error
	return relations, err
}

// GetMarketInfoByCompanyID は指定企業の市場情報を返す。
func (r *CompanyRelationRepository) GetMarketInfoByCompanyID(companyID uint) (*models.CompanyMarketInfo, error) {
	var info models.CompanyMarketInfo
	err := r.db.Where("company_id = ?", companyID).First(&info).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// UpsertMarketInfo は company_market_info を upsert する。
func (r *CompanyRelationRepository) UpsertMarketInfo(info *models.CompanyMarketInfo) error {
	if info == nil {
		return fmt.Errorf("market info is nil")
	}
	var existing models.CompanyMarketInfo
	err := r.db.Where("company_id = ?", info.CompanyID).First(&existing).Error
	if err == nil {
		existing.MarketType = info.MarketType
		existing.IsListed = info.IsListed
		existing.StockCode = info.StockCode
		if info.MarketCap != nil {
			existing.MarketCap = info.MarketCap
		}
		if info.ListingDate != nil {
			existing.ListingDate = info.ListingDate
		}
		return r.db.Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.db.Create(info).Error
}
