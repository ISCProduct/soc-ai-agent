package interfaces

import "Backend/internal/models"

// InterviewSessionCounter IntegratedProfileControllerが使うリポジトリの最小インターフェース
type InterviewSessionCounter interface {
	CountByUser(userID uint) (int64, error)
}

// ResumeDocumentFinder IntegratedProfileControllerが使うリポジトリの最小インターフェース
type ResumeDocumentFinder interface {
	FindDocumentsByUserID(userID uint) ([]models.ResumeDocument, error)
}
