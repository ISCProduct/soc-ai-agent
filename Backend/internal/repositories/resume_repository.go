package repositories

import (
	"errors"

	"Backend/internal/models"

	"gorm.io/gorm"
)

type ResumeRepository struct {
	db *gorm.DB
}

func NewResumeRepository(db *gorm.DB) *ResumeRepository {
	return &ResumeRepository{db: db}
}

func (r *ResumeRepository) CreateDocument(doc *models.ResumeDocument) error {
	if err := fillOrganizationID(r.db, doc.UserID, &doc.OrganizationID); err != nil {
		return err
	}
	return r.db.Create(doc).Error
}

func (r *ResumeRepository) UpdateDocument(doc *models.ResumeDocument) error {
	return r.db.Save(doc).Error
}

func (r *ResumeRepository) FindDocumentByID(id uint) (*models.ResumeDocument, error) {
	var doc models.ResumeDocument
	if err := r.db.First(&doc, id).Error; err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *ResumeRepository) ReplaceTextBlocks(documentID uint, blocks []models.ResumeTextBlock) error {
	if err := r.db.Where("document_id = ?", documentID).Delete(&models.ResumeTextBlock{}).Error; err != nil {
		return err
	}
	if len(blocks) == 0 {
		return nil
	}
	return r.db.Create(&blocks).Error
}

func (r *ResumeRepository) FindTextBlocks(documentID uint) ([]models.ResumeTextBlock, error) {
	var blocks []models.ResumeTextBlock
	if err := r.db.Where("document_id = ?", documentID).
		Order("page_number ASC, block_index ASC").
		Find(&blocks).Error; err != nil {
		return nil, err
	}
	return blocks, nil
}

func (r *ResumeRepository) CreateReview(review *models.ResumeReview) error {
	return r.db.Create(review).Error
}

func (r *ResumeRepository) ReplaceReviewItems(reviewID uint, items []models.ResumeReviewItem) error {
	if err := r.db.Where("review_id = ?", reviewID).Delete(&models.ResumeReviewItem{}).Error; err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	return r.db.Create(&items).Error
}

func (r *ResumeRepository) FindDocumentsByUserID(userID uint) ([]models.ResumeDocument, error) {
	var docs []models.ResumeDocument
	if err := r.db.Where("user_id = ?", userID).Find(&docs).Error; err != nil {
		return nil, err
	}
	return docs, nil
}

// FindLatestDocumentWithReview はユーザーの最新 ResumeDocument と、それに紐づく最新 ResumeReview を返す。
// 「最新」は作成日時の降順とし、同一時刻の場合は ID の降順で決める(#1030)。
//
// 戻り値の組み合わせは3通り。
//   - (nil, nil, nil): 履歴書を一度もアップロードしていない
//   - (doc, nil, nil): アップロード済みだがレビューが未生成(処理中)
//   - (doc, review, nil): レビュー済み
func (r *ResumeRepository) FindLatestDocumentWithReview(userID uint) (*models.ResumeDocument, *models.ResumeReview, error) {
	var doc models.ResumeDocument
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC, id DESC").
		First(&doc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	var review models.ResumeReview
	err = r.db.Where("document_id = ?", doc.ID).
		Order("created_at DESC, id DESC").
		First(&review).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &doc, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return &doc, &review, nil
}

func (r *ResumeRepository) FindReviewItems(reviewID uint) ([]models.ResumeReviewItem, error) {
	var items []models.ResumeReviewItem
	if err := r.db.Where("review_id = ?", reviewID).
		Order("page_number ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
