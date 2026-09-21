package admin_test

// AdminCrawlControllerのHTTPハンドラーテスト (Issue #431)
//
// 実行: cd Backend && go test ./internal/controllers/admin/... -run "AdminCrawl" -v

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	admincontrollers "Backend/internal/controllers/admin"
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/models"

	"github.com/stretchr/testify/mock"
)

func newAdminCrawlController(crawlSvc *mocks.CrawlServiceMock, audit *mocks.AuditLogServiceMock) *admincontrollers.AdminCrawlController {
	return admincontrollers.NewAdminCrawlController(crawlSvc, audit)
}

// ===== ListSources =====

func TestAdminCrawlController_Sources_List_ServiceError(t *testing.T) {
	svc := &mocks.CrawlServiceMock{}
	svc.On("ListSources").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/crawl-sources", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminCrawlController(svc, nil).ListSources, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

func TestAdminCrawlController_Sources_List_Success(t *testing.T) {
	svc := &mocks.CrawlServiceMock{}
	sources := []models.CrawlSource{{Name: "Test Source"}}
	svc.On("ListSources").Return(sources, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/crawl-sources", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminCrawlController(svc, nil).ListSources, testsupport.NewCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

// ===== CreateSource =====

func TestAdminCrawlController_Sources_Create_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/crawl-sources", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, admincontrollers.NewAdminCrawlController(nil, nil).CreateSource, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestAdminCrawlController_Sources_Create_ServiceError(t *testing.T) {
	svc := &mocks.CrawlServiceMock{}
	svc.On("CreateSource", mock.Anything).Return(nil, errors.New("validation error"))

	body, _ := json.Marshal(map[string]string{"name": "Test"})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/crawl-sources", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminCrawlController(svc, nil).CreateSource, testsupport.NewCtx(req, rec), http.StatusBadRequest)
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
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminCrawlController(svc, audit).CreateSource, testsupport.NewCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

// ===== UpdateSource =====

func TestAdminCrawlController_SourceDetail_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/admin/crawl-sources/abc", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("abc")
	testsupport.AssertStatus(t, admincontrollers.NewAdminCrawlController(nil, nil).UpdateSource, ctx, http.StatusBadRequest)
}

func TestAdminCrawlController_SourceDetail_Update_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/admin/crawl-sources/1", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, admincontrollers.NewAdminCrawlController(nil, nil).UpdateSource, ctx, http.StatusBadRequest)
}

func TestAdminCrawlController_SourceDetail_Update_Success(t *testing.T) {
	svc := &mocks.CrawlServiceMock{}
	audit := &mocks.AuditLogServiceMock{}
	source := &models.CrawlSource{Name: "Updated"}
	svc.On("UpdateSource", uint(1), mock.Anything).Return(source, nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	body, _ := json.Marshal(map[string]string{"name": "Updated"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/crawl-sources/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newAdminCrawlController(svc, audit).UpdateSource, ctx, http.StatusOK)
	svc.AssertExpectations(t)
}

// ===== Runs =====

func TestAdminCrawlController_Runs_ServiceError(t *testing.T) {
	svc := &mocks.CrawlServiceMock{}
	svc.On("ListRuns", uint(0), 20).Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/crawl-runs", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminCrawlController(svc, nil).Runs, testsupport.NewCtx(req, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

func TestAdminCrawlController_Runs_Success(t *testing.T) {
	svc := &mocks.CrawlServiceMock{}
	runs := []models.CrawlRun{{Status: "success"}}
	svc.On("ListRuns", uint(0), 20).Return(runs, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/crawl-runs", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAdminCrawlController(svc, nil).Runs, testsupport.NewCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

// ===== RunSource =====

func TestAdminCrawlController_RunSource_ServiceError(t *testing.T) {
	svc := &mocks.CrawlServiceMock{}
	svc.On("RunSource", uint(1)).Return(nil, errors.New("run failed"))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/crawl-sources/1/run", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newAdminCrawlController(svc, nil).RunSource, ctx, http.StatusBadRequest)
	svc.AssertExpectations(t)
}

func TestAdminCrawlController_RunSource_Success(t *testing.T) {
	svc := &mocks.CrawlServiceMock{}
	audit := &mocks.AuditLogServiceMock{}
	run := &models.CrawlRun{Status: "success"}
	svc.On("RunSource", uint(1)).Return(run, nil)
	audit.On("Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/crawl-sources/1/run", nil)
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("1")
	testsupport.AssertStatus(t, newAdminCrawlController(svc, audit).RunSource, ctx, http.StatusOK)
	svc.AssertExpectations(t)
}
