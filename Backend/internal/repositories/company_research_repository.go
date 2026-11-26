package repositories

import (
	"context"
	"errors"
	"time"

	"Backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CompanyResearchRepository は企業研究AIの材料（文章と数字）を扱う（SOCAIAGENT-724）。
type CompanyResearchRepository struct {
	db *gorm.DB
}

func NewCompanyResearchRepository(db *gorm.DB) *CompanyResearchRepository {
	return &CompanyResearchRepository{db: db}
}

// UpsertDocument は本文が変わったときだけ書き込み、変わったかを返す。
func (r *CompanyResearchRepository) UpsertDocument(ctx context.Context, doc *models.CompanyDocument) (bool, error) {
	changed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cur models.CompanyDocument
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("company_id = ? AND source_type = ? AND source_ref = ?", doc.CompanyID, doc.SourceType, doc.SourceRef).
			Take(&cur).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			changed = true
			return tx.Create(doc).Error
		case err != nil:
			return err
		case cur.ContentHash == doc.ContentHash:
			// 本文が同じなら取得日も動かさない（「いつの内容か」は本文が変わった日で表す）。
			*doc = cur
			return nil
		}
		changed = true
		doc.ID = cur.ID
		doc.IndexedAt = nil // 本文が変わったので RAG に登録し直す
		return tx.Model(&cur).Select("title", "url", "as_of", "fetched_at", "content_hash", "body", "indexed_at").Updates(doc).Error
	})
	return changed, err
}

func (r *CompanyResearchRepository) ListDocuments(ctx context.Context, companyID uint) ([]models.CompanyDocument, error) {
	var docs []models.CompanyDocument
	err := r.db.WithContext(ctx).Where("company_id = ?", companyID).Order("source_type, source_ref").Find(&docs).Error
	return docs, err
}

func (r *CompanyResearchRepository) UpsertFacts(ctx context.Context, facts []models.CompanyFact) error {
	if len(facts) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "company_id"}, {Name: "field"}, {Name: "source_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "unit", "source_url", "as_of", "fetched_at", "updated_at"}),
	}).Create(&facts).Error
}

func (r *CompanyResearchRepository) ListFacts(ctx context.Context, companyID uint) ([]models.CompanyFact, error) {
	var facts []models.CompanyFact
	err := r.db.WithContext(ctx).Where("company_id = ?", companyID).Order("field, source_type").Find(&facts).Error
	return facts, err
}

// ListDocumentsToIndex は RAG にまだ登録していない文章を、企業ごとにまとまるよう並べて返す。
func (r *CompanyResearchRepository) ListDocumentsToIndex(ctx context.Context, limit int) ([]models.CompanyDocument, error) {
	var docs []models.CompanyDocument
	err := r.db.WithContext(ctx).Where("indexed_at IS NULL").Order("company_id, id").Limit(limit).Find(&docs).Error
	return docs, err
}

// MarkIndexed は RAG に登録した文章に登録日時を入れる。
func (r *CompanyResearchRepository) MarkIndexed(ctx context.Context, ids []uint, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&models.CompanyDocument{}).Where("id IN ?", ids).Update("indexed_at", at).Error
}
