package repository

import "Backend/internal/models"

// ResumeRepository は職務経歴書の永続化インターフェース。
type ResumeRepository interface {
	CreateDocument(doc *models.ResumeDocument) error
	UpdateDocument(doc *models.ResumeDocument) error
	// FindDocumentByIDForUser はクエリ自体を user_id でスコープする（#1156 多層防御）。
	// 主キーだけで引く版（FindDocumentByID）は、誤って使われて IDOR にならないよう #2003 で削除した。
	// user_id でスコープしない取得メソッドをここに足さない。
	FindDocumentByIDForUser(id, userID uint) (*models.ResumeDocument, error)
	ReplaceTextBlocks(documentID uint, blocks []models.ResumeTextBlock) error
	CreateReview(review *models.ResumeReview) error
	ReplaceReviewItems(reviewID uint, items []models.ResumeReviewItem) error
	// FindLatestDocumentWithReview は最新ドキュメントと、それに紐づく最新レビューを返す(#1030)。
	FindLatestDocumentWithReview(userID uint) (*models.ResumeDocument, *models.ResumeReview, error)
}
