package interfaces

import (
	"Backend/internal/models"
	"Backend/internal/services"
	"context"
	"mime/multipart"
	"net/http"
)

type ResumeService interface {
	Upload(userID uint, sessionID, sourceType, sourceURL string, fileHeader *multipart.FileHeader) (*services.ResumeUploadResult, error)
	EnsureDocumentOwner(documentID uint, requestingUserID uint) error
	ReviewDocument(documentID uint, requestingUserID uint, companyName, jobTitle, candidateType string) (*models.ResumeReview, []models.ResumeReviewItem, error)
	ReviewDocumentStream(ctx context.Context, documentID uint, requestingUserID uint, companyName, jobTitle, candidateType string, w http.ResponseWriter) error
	OpenAnnotatedFile(documentID uint, requestingUserID uint) (*services.AnnotatedFile, error)
}
