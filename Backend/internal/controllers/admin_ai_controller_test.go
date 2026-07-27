package controllers

import (
	"Backend/internal/middleware"
	"Backend/internal/services"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

// mock service
type mockAdminAIService struct {
	getSummaryFn    func() (*services.AdminAIMetrics, error)
	triggerReembedFn func() error
	forceResyncFn   func() error
}

func (m *mockAdminAIService) GetSummary(ctxReq interface{}) (*services.AdminAIMetrics, error) {
	if m.getSummaryFn != nil {
		return m.getSummaryFn()
	}
	return &services.AdminAIMetrics{}, nil
}

func (m *mockAdminAIService) TriggerReembed(ctxReq interface{}) error {
	if m.triggerReembedFn != nil {
		return m.triggerReembedFn()
	}
	return nil
}

func (m *mockAdminAIService) ForceResync(ctxReq interface{}) error {
	if m.forceResyncFn != nil {
		return m.forceResyncFn()
	}
	return nil
}

// helpers
func newAdminAITestServer(svc *mockAdminAIService) (*echo.Echo, *AdminAIController) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomHTTPErrorHandler
	// wrap service into expected type
	wrapped := &services.AdminAIService{}
	// we will not use wrapped methods during test because controller calls our mock via interface,
	// so create controller manually and inject mock via field (using type assertion)
	ctrl := &AdminAIController{service: nil}
	// replace service field via type interface by using reflection is heavy; instead create controller directly
	// but controller expects *services.AdminAIService; to avoid changing controller, we'll create a thin adapter
	// Adapter implements the methods the controller uses by delegating to mock
	adapter := &adminAIServiceAdapter{mock: svc}
	ctrl = &AdminAIController{service: adapter}
	return e, ctrl
}

// adapter to satisfy minimal interface used by controller
type adminAIServiceAdapter struct{
	mock *mockAdminAIService
}

func (a *adminAIServiceAdapter) GetSummary(ctxReq interface{}) (*services.AdminAIMetrics, error) {
	return a.mock.getSummaryFn()
}
func (a *adminAIServiceAdapter) TriggerReembed(ctxReq interface{}) error {
	return a.mock.triggerReembedFn()
}
func (a *adminAIServiceAdapter) ForceResync(ctxReq interface{}) error {
	return a.mock.forceResyncFn()
}

// serve helpers copied pattern
func serveGet(e *echo.Echo, path string, handler echo.HandlerFunc) *httptest.ResponseRecorder {
	e.GET(path, handler)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func servePost(e *echo.Echo, path, body string, handler echo.HandlerFunc) *httptest.ResponseRecorder {
	e.POST(path, handler)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestAdminAI_Summary_Success(t *testing.T) {
	now := time.Now().UTC()
	mock := &mockAdminAIService{
		getSummaryFn: func() (*services.AdminAIMetrics, error) {
			return &services.AdminAIMetrics{
				CollectionCount: 5,
				LastUpdated:     now,
				CacheHitRate:    0.85,
				EstimatedSaveUSD: 123.45,
			}, nil
		},
	}
	e, ctrl := newAdminAITestServer(mock)
	rec := serveGet(e, "/admin/ai-rag/summary", ctrl.Summary)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp services.AdminAIMetrics
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json unmarshal failed: %v (body=%s)", err, rec.Body.String())
	}
	if resp.CollectionCount != 5 {
		t.Errorf("collection_count = %d, want %d", resp.CollectionCount, 5)
	}
}

func TestAdminAI_Reembed_TriggersAccepted(t *testing.T) {
	called := false
	mock := &mockAdminAIService{
		triggerReembedFn: func() error { called = true; return nil },
	}
	e, ctrl := newAdminAITestServer(mock)
	rec := servePost(e, "/admin/ai-rag/reembed", `{}` , ctrl.Reembed)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if !called {
		t.Errorf("expected TriggerReembed to be called")
	}
}

func TestAdminAI_ForceResync_TriggersAccepted(t *testing.T) {
	called := false
	mock := &mockAdminAIService{
		forceResyncFn: func() error { called = true; return nil },
	}
	e, ctrl := newAdminAITestServer(mock)
	rec := servePost(e, "/admin/ai-rag/force-resync", `{}` , ctrl.ForceResync)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if !called {
		t.Errorf("expected ForceResync to be called")
	}
}
