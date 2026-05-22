package controllers_test

// ResumeControllerのHTTPハンドラーテスト (Issue #397/#422)
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

func newResumeControllerWithMock(svc *mocks.ResumeServiceMock) *controllers.ResumeController {
	return controllers.NewResumeController(svc)
}

// ---- Upload ----

func TestResumeController_Upload_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/resumes/upload", nil)
			w := httptest.NewRecorder()
			controllers.NewResumeController(nil).Upload(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestResumeController_Upload_InvalidForm(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", bytes.NewBufferString("not-multipart"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	controllers.NewResumeController(nil).Upload(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResumeController_Upload_MissingUserID(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	controllers.NewResumeController(nil).Upload(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResumeController_Upload_Unauthorized(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("user_id", "1")
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	controllers.NewResumeController(nil).Upload(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestResumeController_Upload_Forbidden(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("user_id", "99")
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	controllers.NewResumeController(nil).Upload(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
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
	w := httptest.NewRecorder()
	newResumeControllerWithMock(svc).Upload(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ---- Review ----

func TestResumeController_Review_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/resumes/review?document_id=1", nil)
			w := httptest.NewRecorder()
			controllers.NewResumeController(nil).Review(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestResumeController_Review_MissingDocumentID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review", nil)
	w := httptest.NewRecorder()
	controllers.NewResumeController(nil).Review(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResumeController_Review_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review?document_id=1", nil)
	w := httptest.NewRecorder()
	controllers.NewResumeController(nil).Review(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestResumeController_Review_Forbidden(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	svc.On("EnsureDocumentOwner", uint(1), uint(1)).Return(services.ErrForbidden)

	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review?document_id=1", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	newResumeControllerWithMock(svc).Review(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
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
	w := httptest.NewRecorder()
	newResumeControllerWithMock(svc).Review(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ---- Annotated ----

func TestResumeController_Annotated_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/resumes/annotated?document_id=1", nil)
			w := httptest.NewRecorder()
			controllers.NewResumeController(nil).Annotated(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestResumeController_Annotated_MissingDocumentID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/resumes/annotated", nil)
	w := httptest.NewRecorder()
	controllers.NewResumeController(nil).Annotated(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResumeController_Annotated_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/resumes/annotated?document_id=1", nil)
	w := httptest.NewRecorder()
	controllers.NewResumeController(nil).Annotated(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestResumeController_Annotated_Forbidden(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	svc.On("OpenAnnotatedFile", uint(1), uint(1)).Return(nil, services.ErrForbidden)

	req := httptest.NewRequest(http.MethodGet, "/api/resumes/annotated?document_id=1", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	newResumeControllerWithMock(svc).Annotated(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertExpectations(t)
}

func TestResumeController_Annotated_NotFound(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	svc.On("OpenAnnotatedFile", uint(1), uint(1)).Return(nil, errors.New("file not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/resumes/annotated?document_id=1", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	newResumeControllerWithMock(svc).Annotated(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

// ---- ReviewStream ----

func TestResumeController_ReviewStream_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/resumes/review/stream?document_id=1", nil)
			w := httptest.NewRecorder()
			controllers.NewResumeController(nil).ReviewStream(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestResumeController_ReviewStream_MissingDocumentID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review/stream", nil)
	w := httptest.NewRecorder()
	controllers.NewResumeController(nil).ReviewStream(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResumeController_ReviewStream_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review/stream?document_id=1", nil)
	w := httptest.NewRecorder()
	controllers.NewResumeController(nil).ReviewStream(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestResumeController_ReviewStream_Forbidden(t *testing.T) {
	svc := &mocks.ResumeServiceMock{}
	svc.On("EnsureDocumentOwner", uint(1), uint(1)).Return(services.ErrForbidden)

	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review/stream?document_id=1", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	newResumeControllerWithMock(svc).ReviewStream(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertExpectations(t)
}
