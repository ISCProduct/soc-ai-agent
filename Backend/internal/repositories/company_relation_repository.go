package repositories

import (
	"Backend/internal/models"
	"errors"

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
