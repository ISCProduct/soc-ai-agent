package controllers_test

// AdminCrawlControllerのHTTPハンドラーテスト (Issue #431)
//
// 実行: cd Backend && go test ./test/controllers/... -run "AdminCrawl" -v

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	"Backend/internal/models"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newAdminCrawlController(crawlSvc *mocks.CrawlServiceMock, audit *mocks.AuditLogServiceMock) *controllers.AdminCrawlController {
	return controllers.NewAdminCrawlController(crawlSvc, audit)
}

// ===== Sources =====

func TestAdminCrawlController_Sources_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/crawl-sources", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminCrawlController(nil, nil).Sources(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminCrawlController_Sources_List_ServiceError(t *testing.T) {
	svc := &mocks.CrawlServiceMock{}
	svc.On("ListSources").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/crawl-sources", nil)
	w := httptest.NewRecorder()
	newAdminCrawlController(svc, nil).Sources(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminCrawlController_Sources_List_Success(t *testing.T) {
	svc := &mocks.CrawlServiceMock{}
	sources := []models.CrawlSource{{Name: "Test Source"}}
	svc.On("ListSources").Return(sources, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/crawl-sources", nil)
	w := httptest.NewRecorder()
	newAdminCrawlController(svc, nil).Sources(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminCrawlController_Sources_Create_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/crawl-sources", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	controllers.NewAdminCrawlController(nil, nil).Sources(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminCrawlController_Sources_Create_ServiceError(t *testing.T) {
	svc := &mocks.CrawlServiceMock{}
	svc.On("CreateSource", mock.Anything).Return(nil, errors.New("validation error"))

	body, _ := json.Marshal(map[string]string{"name": "Test"})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/crawl-sources", bytes.NewReader(body))
	w := httptest.NewRecorder()
	newAdminCrawlController(svc, nil).Sources(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminCrawlController_Sources_Create_Success(t *testing.T) {
	svc := &mocks.CrawlServiceMock{}
	audit := &mocks.AuditLogServiceMock{}
	source := &models.CrawlSource{Name: "Test Source"}
	svc.On("CreateSource", mock.Anything).Return(source, nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	body, _ := json.Marshal(map[string]string{"name": "Test Source"})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/crawl-sources", bytes.NewReader(body))
	w := httptest.NewRecorder()
	newAdminCrawlController(svc, audit).Sources(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ===== SourceDetail =====

func TestAdminCrawlController_SourceDetail_EmptyID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/admin/crawl-sources/", nil)
	req.URL.Path = "/api/admin/crawl-sources/"
	w := httptest.NewRecorder()
	controllers.NewAdminCrawlController(nil, nil).SourceDetail(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminCrawlController_SourceDetail_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/admin/crawl-sources/abc", nil)
	req.URL.Path = "/api/admin/crawl-sources/abc"
	w := httptest.NewRecorder()
	controllers.NewAdminCrawlController(nil, nil).SourceDetail(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminCrawlController_SourceDetail_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/crawl-sources/1", nil)
	req.URL.Path = "/api/admin/crawl-sources/1"
	w := httptest.NewRecorder()
	controllers.NewAdminCrawlController(nil, nil).SourceDetail(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminCrawlController_SourceDetail_Update_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/admin/crawl-sources/1", bytes.NewBufferString("not-json"))
	req.URL.Path = "/api/admin/crawl-sources/1"
	w := httptest.NewRecorder()
	controllers.NewAdminCrawlController(nil, nil).SourceDetail(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminCrawlController_SourceDetail_Update_Success(t *testing.T) {
	svc := &mocks.CrawlServiceMock{}
	audit := &mocks.AuditLogServiceMock{}
	source := &models.CrawlSource{Name: "Updated"}
	svc.On("UpdateSource", uint(1), mock.Anything).Return(source, nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	body, _ := json.Marshal(map[string]string{"name": "Updated"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/crawl-sources/1", bytes.NewReader(body))
	req.URL.Path = "/api/admin/crawl-sources/1"
	w := httptest.NewRecorder()
	newAdminCrawlController(svc, audit).SourceDetail(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ===== Runs =====

func TestAdminCrawlController_Runs_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/crawl-runs", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminCrawlController(nil, nil).Runs(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminCrawlController_Runs_ServiceError(t *testing.T) {
	svc := &mocks.CrawlServiceMock{}
	svc.On("ListRuns", uint(0), 20).Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/crawl-runs", nil)
	w := httptest.NewRecorder()
	newAdminCrawlController(svc, nil).Runs(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminCrawlController_Runs_Success(t *testing.T) {
	svc := &mocks.CrawlServiceMock{}
	runs := []models.CrawlRun{{Status: "success"}}
	svc.On("ListRuns", uint(0), 20).Return(runs, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/crawl-runs", nil)
	w := httptest.NewRecorder()
	newAdminCrawlController(svc, nil).Runs(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ===== runSource (via SourceDetail /run suffix) =====

func TestAdminCrawlController_RunSource_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/crawl-sources/1/run", nil)
	req.URL.Path = "/api/admin/crawl-sources/1/run"
	w := httptest.NewRecorder()
	controllers.NewAdminCrawlController(nil, nil).SourceDetail(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminCrawlController_RunSource_ServiceError(t *testing.T) {
	svc := &mocks.CrawlServiceMock{}
	svc.On("RunSource", uint(1)).Return(nil, errors.New("run failed"))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/crawl-sources/1/run", nil)
	req.URL.Path = "/api/admin/crawl-sources/1/run"
	w := httptest.NewRecorder()
	newAdminCrawlController(svc, nil).SourceDetail(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestAdminCrawlController_RunSource_Success(t *testing.T) {
	svc := &mocks.CrawlServiceMock{}
	audit := &mocks.AuditLogServiceMock{}
	run := &models.CrawlRun{Status: "success"}
	svc.On("RunSource", uint(1)).Return(run, nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/crawl-sources/1/run", nil)
	req.URL.Path = "/api/admin/crawl-sources/1/run"
	w := httptest.NewRecorder()
	newAdminCrawlController(svc, audit).SourceDetail(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}
