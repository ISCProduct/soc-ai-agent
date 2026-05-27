package mocks

import (
	"Backend/internal/models"
	"Backend/internal/services"
	"context"
	"mime/multipart"
	"net/http"

	"github.com/stretchr/testify/mock"
)

type ResumeServiceMock struct {
	mock.Mock
}

func (m *ResumeServiceMock) Upload(userID uint, sessionID, sourceType, sourceURL string, fileHeader *multipart.FileHeader) (*services.ResumeUploadResult, error) {
	args := m.Called(userID, sessionID, sourceType, sourceURL, fileHeader)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.ResumeUploadResult), args.Error(1)
}

func (m *ResumeServiceMock) EnsureDocumentOwner(documentID uint, requestingUserID uint) error {
	return m.Called(documentID, requestingUserID).Error(0)
}

func (m *ResumeServiceMock) ReviewDocument(documentID uint, requestingUserID uint, companyName, jobTitle, candidateType string) (*models.ResumeReview, []models.ResumeReviewItem, error) {
	args := m.Called(documentID, requestingUserID, companyName, jobTitle, candidateType)
	var review *models.ResumeReview
	var items []models.ResumeReviewItem
	if args.Get(0) != nil {
		review = args.Get(0).(*models.ResumeReview)
	}
	if args.Get(1) != nil {
		items = args.Get(1).([]models.ResumeReviewItem)
	}
	return review, items, args.Error(2)
}

func (m *ResumeServiceMock) ReviewDocumentStream(ctx context.Context, documentID uint, requestingUserID uint, companyName, jobTitle, candidateType string, w http.ResponseWriter) error {
	return m.Called(ctx, documentID, requestingUserID, companyName, jobTitle, candidateType, w).Error(0)
}

func (m *ResumeServiceMock) OpenAnnotatedFile(documentID uint, requestingUserID uint) (*services.AnnotatedFile, error) {
	args := m.Called(documentID, requestingUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.AnnotatedFile), args.Error(1)
}
