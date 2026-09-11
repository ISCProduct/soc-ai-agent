package interfaces

import (
	"Backend/internal/models"
	"Backend/internal/services/resume"
	"context"
	"mime/multipart"
	"net/http"
)

type ResumeService interface {
	Upload(userID uint, sessionID, sourceType, sourceURL string, fileHeader *multipart.FileHeader) (*resume.ResumeUploadResult, error)
	EnsureDocumentOwner(documentID uint, requestingUserID uint) error
	ReviewDocument(documentID uint, requestingUserID uint, companyName, jobTitle, candidateType string) (*models.ResumeReview, []models.ResumeReviewItem, error)
	ReviewDocumentStream(ctx context.Context, documentID uint, requestingUserID uint, companyName, jobTitle, candidateType string, w http.ResponseWriter) error
	OpenAnnotatedFile(documentID uint, requestingUserID uint) (*resume.AnnotatedFile, error)
	// GetResumeStatus は履歴書リマインダーの表示要否を返す(#1030)。
	GetResumeStatus(userID uint) (*resume.ResumeStatus, error)
}
