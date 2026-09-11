package repository

import "Backend/internal/models"

// ResumeRepository は職務経歴書の永続化インターフェース。
type ResumeRepository interface {
	CreateDocument(doc *models.ResumeDocument) error
	UpdateDocument(doc *models.ResumeDocument) error
	FindDocumentByID(id uint) (*models.ResumeDocument, error)
	ReplaceTextBlocks(documentID uint, blocks []models.ResumeTextBlock) error
	FindTextBlocks(documentID uint) ([]models.ResumeTextBlock, error)
	CreateReview(review *models.ResumeReview) error
	ReplaceReviewItems(reviewID uint, items []models.ResumeReviewItem) error
	FindReviewItems(reviewID uint) ([]models.ResumeReviewItem, error)
	// FindLatestDocumentWithReview は最新ドキュメントと、それに紐づく最新レビューを返す(#1030)。
	FindLatestDocumentWithReview(userID uint) (*models.ResumeDocument, *models.ResumeReview, error)
}
