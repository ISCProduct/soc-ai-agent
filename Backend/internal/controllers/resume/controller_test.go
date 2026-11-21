package resume_test

// ResumeControllerのHTTPハンドラーテスト
//
// 実行: cd Backend && go test ./internal/controllers/resume/... -run Resume -v

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/controllers/mocks"
	resumecontrollers "Backend/internal/controllers/resume"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"
	"Backend/internal/services/resume"
	"Backend/internal/services/shared"

	"github.com/stretchr/testify/assert"
)

func newResumeController(svc *mocks.ResumeServiceMock) *resumecontrollers.ResumeController {
	return resumecontrollers.NewResumeController(svc)
}

// ---- Upload ----

func TestResumeController_Upload_InvalidForm(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", bytes.NewBufferString("not-multipart"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, resumecontrollers.NewResumeController(nil).Upload, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestResumeController_Upload_MissingUserID(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, resumecontrollers.NewResumeController(nil).Upload, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestResumeController_Upload_Unauthorized(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("user_id", "1")
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, resumecontrollers.NewResumeController(nil).Upload, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestResumeController_Upload_Forbidden(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("user_id", "99")
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, resumecontrollers.NewResumeController(nil).Upload, testsupport.NewCtx(req, rec), http.StatusForbidden)
}

func TestResumeController_Upload_Success(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	doc := &models.ResumeDocument{SourceType: "text"}
	svc.On("Upload", uint(1), "sess-1", "text", "", (*multipart.FileHeader)(nil)).
		Return(&resume.ResumeUploadResult{Document: doc}, nil)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("user_id", "1")
	writer.WriteField("session_id", "sess-1")
	writer.WriteField("source_type", "text")
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newResumeController(svc).Upload, testsupport.NewCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

// ---- Review ----

func TestResumeController_Review_MissingDocumentID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, resumecontrollers.NewResumeController(nil).Review, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestResumeController_Review_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review?document_id=1", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, resumecontrollers.NewResumeController(nil).Review, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestResumeController_Review_Forbidden(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	svc.On("EnsureDocumentOwner", uint(1), uint(1)).Return(shared.ErrForbidden)

	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review?document_id=1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newResumeController(svc).Review, testsupport.NewCtx(req, rec), http.StatusForbidden)
	svc.AssertExpectations(t)
}

func TestResumeController_Review_Success(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	score := 80
	review := &models.ResumeReview{Score: &score}
	items := []models.ResumeReviewItem{{Severity: "info", Message: "構成良好"}}
	svc.On("EnsureDocumentOwner", uint(1), uint(1)).Return(nil)
	svc.On("ReviewDocument", uint(1), uint(1), "", "", "").Return(review, items, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review?document_id=1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newResumeController(svc).Review, testsupport.NewCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

// ---- Annotated ----

func TestResumeController_Annotated_MissingDocumentID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/resumes/annotated", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, resumecontrollers.NewResumeController(nil).Annotated, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestResumeController_Annotated_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/resumes/annotated?document_id=1", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, resumecontrollers.NewResumeController(nil).Annotated, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestResumeController_Annotated_Forbidden(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	svc.On("OpenAnnotatedFile", uint(1), uint(1)).Return(nil, shared.ErrForbidden)

	req := httptest.NewRequest(http.MethodGet, "/api/resumes/annotated?document_id=1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newResumeController(svc).Annotated, testsupport.NewCtx(req, rec), http.StatusForbidden)
	svc.AssertExpectations(t)
}

func TestResumeController_Annotated_NotFound(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	svc.On("OpenAnnotatedFile", uint(1), uint(1)).Return(nil, errors.New("file not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/resumes/annotated?document_id=1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newResumeController(svc).Annotated, testsupport.NewCtx(req, rec), http.StatusNotFound)
	svc.AssertExpectations(t)
}

// ---- Document（レビュー画面が描く、PDFに揃えた元の書類） ----

func TestResumeController_Document_MissingDocumentID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/resume/document", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, resumecontrollers.NewResumeController(nil).Document, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestResumeController_Document_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/resume/document?document_id=1", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, resumecontrollers.NewResumeController(nil).Document, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

// 他人の書類は見せない
func TestResumeController_Document_Forbidden(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	svc.On("OpenNormalizedFile", uint(1), uint(2)).Return(nil, shared.ErrForbidden)

	req := httptest.NewRequest(http.MethodGet, "/api/resume/document?document_id=1", nil)
	req = testsupport.WithUserID(req, 2)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newResumeController(svc).Document, testsupport.NewCtx(req, rec), http.StatusForbidden)
	svc.AssertExpectations(t)
}

func TestResumeController_Document_NotReady(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	svc.On("OpenNormalizedFile", uint(1), uint(1)).Return(nil, errors.New("document file not ready"))

	req := httptest.NewRequest(http.MethodGet, "/api/resume/document?document_id=1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newResumeController(svc).Document, testsupport.NewCtx(req, rec), http.StatusNotFound)
	svc.AssertExpectations(t)
}

func TestResumeController_Document_ServesPDFWithoutSharedCache(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	body := "%PDF-1.4 test"
	svc.On("OpenNormalizedFile", uint(1), uint(1)).Return(&resume.AnnotatedFile{
		Reader:      strings.NewReader(body),
		Size:        int64(len(body)),
		ContentType: "application/pdf",
		Filename:    "normalized.pdf",
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/resume/document?document_id=1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newResumeController(svc).Document, testsupport.NewCtx(req, rec), http.StatusOK)
	if got := rec.Header().Get("Content-Type"); got != "application/pdf" {
		t.Errorf("Content-Type=%q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control=%q", got)
	}
	if rec.Body.String() != body {
		t.Errorf("body=%q", rec.Body.String())
	}
	svc.AssertExpectations(t)
}

// ---- ReviewStream ----

func TestResumeController_ReviewStream_MissingDocumentID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review/stream", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, resumecontrollers.NewResumeController(nil).ReviewStream, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestResumeController_ReviewStream_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review/stream?document_id=1", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, resumecontrollers.NewResumeController(nil).ReviewStream, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestResumeController_ReviewStream_Forbidden(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	svc.On("EnsureDocumentOwner", uint(1), uint(1)).Return(shared.ErrForbidden)

	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review/stream?document_id=1", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newResumeController(svc).ReviewStream, testsupport.NewCtx(req, rec), http.StatusForbidden)
	svc.AssertExpectations(t)
}

// writeInternalServerError が使われないためカバレッジ向上目的で残す
var _ = assert.New
