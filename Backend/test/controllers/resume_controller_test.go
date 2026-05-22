package controllers_test

// ResumeControllerのHTTPハンドラーテスト
//
// 実行: cd Backend && go test ./test/controllers/... -run Resume -v

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	"Backend/internal/models"
	"Backend/internal/services"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
)

func newResumeController(svc *mocks.ResumeServiceMock) *controllers.ResumeController {
	return controllers.NewResumeController(svc)
}

// ---- Upload ----

func TestResumeController_Upload_InvalidForm(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", bytes.NewBufferString("not-multipart"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewResumeController(nil).Upload, newCtx(req, rec), http.StatusBadRequest)
}

func TestResumeController_Upload_MissingUserID(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewResumeController(nil).Upload, newCtx(req, rec), http.StatusBadRequest)
}

func TestResumeController_Upload_Unauthorized(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("user_id", "1")
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewResumeController(nil).Upload, newCtx(req, rec), http.StatusUnauthorized)
}

func TestResumeController_Upload_Forbidden(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("user_id", "99")
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewResumeController(nil).Upload, newCtx(req, rec), http.StatusForbidden)
}

func TestResumeController_Upload_Success(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	doc := &models.ResumeDocument{SourceType: "text"}
	svc.On("Upload", uint(1), "sess-1", "text", "", (*multipart.FileHeader)(nil)).
		Return(&services.ResumeUploadResult{Document: doc}, nil)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("user_id", "1")
	writer.WriteField("session_id", "sess-1")
	writer.WriteField("source_type", "text")
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newResumeController(svc).Upload, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

// ---- Review ----

func TestResumeController_Review_MissingDocumentID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewResumeController(nil).Review, newCtx(req, rec), http.StatusBadRequest)
}

func TestResumeController_Review_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review?document_id=1", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewResumeController(nil).Review, newCtx(req, rec), http.StatusUnauthorized)
}

func TestResumeController_Review_Forbidden(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	svc.On("EnsureDocumentOwner", uint(1), uint(1)).Return(services.ErrForbidden)

	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review?document_id=1", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newResumeController(svc).Review, newCtx(req, rec), http.StatusForbidden)
	svc.AssertExpectations(t)
}

func TestResumeController_Review_Success(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	review := &models.ResumeReview{Score: 80}
	items := []models.ResumeReviewItem{{Severity: "info", Message: "構成良好"}}
	svc.On("EnsureDocumentOwner", uint(1), uint(1)).Return(nil)
	svc.On("ReviewDocument", uint(1), uint(1), "", "", "").Return(review, items, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review?document_id=1", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newResumeController(svc).Review, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

// ---- Annotated ----

func TestResumeController_Annotated_MissingDocumentID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/resumes/annotated", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewResumeController(nil).Annotated, newCtx(req, rec), http.StatusBadRequest)
}

func TestResumeController_Annotated_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/resumes/annotated?document_id=1", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewResumeController(nil).Annotated, newCtx(req, rec), http.StatusUnauthorized)
}

func TestResumeController_Annotated_Forbidden(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	svc.On("OpenAnnotatedFile", uint(1), uint(1)).Return(nil, services.ErrForbidden)

	req := httptest.NewRequest(http.MethodGet, "/api/resumes/annotated?document_id=1", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newResumeController(svc).Annotated, newCtx(req, rec), http.StatusForbidden)
	svc.AssertExpectations(t)
}

func TestResumeController_Annotated_NotFound(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	svc.On("OpenAnnotatedFile", uint(1), uint(1)).Return(nil, errors.New("file not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/resumes/annotated?document_id=1", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newResumeController(svc).Annotated, newCtx(req, rec), http.StatusNotFound)
	svc.AssertExpectations(t)
}

// ---- ReviewStream ----

func TestResumeController_ReviewStream_MissingDocumentID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review/stream", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewResumeController(nil).ReviewStream, newCtx(req, rec), http.StatusBadRequest)
}

func TestResumeController_ReviewStream_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review/stream?document_id=1", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewResumeController(nil).ReviewStream, newCtx(req, rec), http.StatusUnauthorized)
}

func TestResumeController_ReviewStream_Forbidden(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	svc.On("EnsureDocumentOwner", uint(1), uint(1)).Return(services.ErrForbidden)

	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review/stream?document_id=1", nil)
	req = withUserID(req, 1)
	rec := httptest.NewRecorder()
	assertStatus(t, newResumeController(svc).ReviewStream, newCtx(req, rec), http.StatusForbidden)
	svc.AssertExpectations(t)
}

// writeInternalServerError が使われないためカバレッジ向上目的で残す
var _ = assert.New
