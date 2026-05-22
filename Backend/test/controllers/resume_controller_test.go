package controllers_test

// ResumeControllerのHTTPハンドラーテスト (Issue #397)
//
// 実行: cd Backend && go test ./test/controllers/... -run Resume -v

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"

	"github.com/stretchr/testify/assert"
)

func newResumeController() *controllers.ResumeController {
	return controllers.NewResumeController(nil)
}

// TestResumeController_Upload_MethodNotAllowed はPOST以外に405を返すことを検証
func TestResumeController_Upload_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/resumes/upload", nil)
			w := httptest.NewRecorder()
			newResumeController().Upload(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestResumeController_Upload_InvalidForm はmultipartパースエラーに400を返すことを検証
func TestResumeController_Upload_InvalidForm(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", bytes.NewBufferString("not-multipart"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newResumeController().Upload(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestResumeController_Upload_MissingUserID はuser_id未指定に400を返すことを検証
func TestResumeController_Upload_MissingUserID(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	newResumeController().Upload(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestResumeController_Upload_InvalidUserID は非数値user_idに400を返すことを検証
func TestResumeController_Upload_InvalidUserID(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("user_id", "not-a-number")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	newResumeController().Upload(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestResumeController_Upload_Unauthorized はuserIDがコンテキストにない場合に401を返すことを検証
func TestResumeController_Upload_Unauthorized(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("user_id", "1")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	newResumeController().Upload(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestResumeController_Upload_Forbidden は異なるユーザーIDで403を返すことを検証
func TestResumeController_Upload_Forbidden(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("user_id", "99")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/resumes/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withUserID(req, 1) // 認証ユーザーは1、リクエストは99
	w := httptest.NewRecorder()
	newResumeController().Upload(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestResumeController_Review_MethodNotAllowed はPOST以外に405を返すことを検証
func TestResumeController_Review_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/resumes/review?document_id=1", nil)
			w := httptest.NewRecorder()
			newResumeController().Review(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestResumeController_Review_MissingDocumentID はdocument_id未指定に400を返すことを検証
func TestResumeController_Review_MissingDocumentID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review", nil)
	w := httptest.NewRecorder()
	newResumeController().Review(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestResumeController_Review_InvalidDocumentID は非数値document_idに400を返すことを検証
func TestResumeController_Review_InvalidDocumentID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review?document_id=abc", nil)
	w := httptest.NewRecorder()
	newResumeController().Review(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestResumeController_Review_Unauthorized はuserIDがコンテキストにない場合に401を返すことを検証
func TestResumeController_Review_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review?document_id=1", nil)
	w := httptest.NewRecorder()
	newResumeController().Review(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestResumeController_Annotated_MethodNotAllowed はGET以外に405を返すことを検証
func TestResumeController_Annotated_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/resumes/annotated?document_id=1", nil)
			w := httptest.NewRecorder()
			newResumeController().Annotated(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestResumeController_Annotated_MissingDocumentID はdocument_id未指定に400を返すことを検証
func TestResumeController_Annotated_MissingDocumentID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/resumes/annotated", nil)
	w := httptest.NewRecorder()
	newResumeController().Annotated(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestResumeController_Annotated_Unauthorized はuserIDがコンテキストにない場合に401を返すことを検証
func TestResumeController_Annotated_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/resumes/annotated?document_id=1", nil)
	w := httptest.NewRecorder()
	newResumeController().Annotated(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestResumeController_ReviewStream_MethodNotAllowed はPOST以外に405を返すことを検証
func TestResumeController_ReviewStream_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/resumes/review/stream?document_id=1", nil)
			w := httptest.NewRecorder()
			newResumeController().ReviewStream(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestResumeController_ReviewStream_MissingDocumentID はdocument_id未指定に400を返すことを検証
func TestResumeController_ReviewStream_MissingDocumentID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review/stream", nil)
	w := httptest.NewRecorder()
	newResumeController().ReviewStream(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestResumeController_ReviewStream_Unauthorized はuserIDがコンテキストにない場合に401を返すことを検証
func TestResumeController_ReviewStream_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/review/stream?document_id=1", nil)
	w := httptest.NewRecorder()
	newResumeController().ReviewStream(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
