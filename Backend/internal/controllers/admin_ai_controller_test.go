package controllers

import (
	"context"
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
	getSummaryFn     func(ctx context.Context) (*services.AdminAIMetrics, error)
	triggerReembedFn func(ctx context.Context) error
	forceResyncFn    func(ctx context.Context) error
}

func (m *mockAdminAIService) GetSummary(ctx context.Context) (*services.AdminAIMetrics, error) {
	if m.getSummaryFn != nil {
		return m.getSummaryFn(ctx)
	}
	return &services.AdminAIMetrics{}, nil
}

func (m *mockAdminAIService) TriggerReembed(ctx context.Context) error {
	if m.triggerReembedFn != nil {
		return m.triggerReembedFn(ctx)
	}
	return nil
}

func (m *mockAdminAIService) ForceResync(ctx context.Context) error {
	if m.forceResyncFn != nil {
		return m.forceResyncFn(ctx)
	}
	return nil
}

// helpers
func newAdminAITestServer(svc *mockAdminAIService) (*echo.Echo, *AdminAIController) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomHTTPErrorHandler
	adapter := &adminAIServiceAdapter{mock: svc}
	ctrl := &AdminAIController{service: adapter}
	return e, ctrl
}

// adapter to satisfy minimal interface used by controller
type adminAIServiceAdapter struct{
	mock *mockAdminAIService
}

func (a *adminAIServiceAdapter) GetSummary(ctx context.Context) (*services.AdminAIMetrics, error) {
	if a.mock.getSummaryFn != nil {
		return a.mock.getSummaryFn(ctx)
	}
	return &services.AdminAIMetrics{}, nil
}
func (a *adminAIServiceAdapter) TriggerReembed(ctx context.Context) error {
	if a.mock.triggerReembedFn != nil {
		return a.mock.triggerReembedFn(ctx)
	}
	return nil
}
func (a *adminAIServiceAdapter) ForceResync(ctx context.Context) error {
	if a.mock.forceResyncFn != nil {
		return a.mock.forceResyncFn(ctx)
	}
	return nil
}

// serve helpers with unique names to avoid collisions
func serveGetAdminAI(e *echo.Echo, path string, handler echo.HandlerFunc) *httptest.ResponseRecorder {
	e.GET(path, handler)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func servePostAdminAI(e *echo.Echo, path, body string, handler echo.HandlerFunc) *httptest.ResponseRecorder {
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
		getSummaryFn: func(ctx context.Context) (*services.AdminAIMetrics, error) {
			return &services.AdminAIMetrics{
				CollectionCount: 5,
				LastUpdated:     now,
				CacheHitRate:    0.85,
				EstimatedSaveUSD: 123.45,
			}, nil
		},
	}
	e, ctrl := newAdminAITestServer(mock)
	rec := serveGetAdminAI(e, "/admin/ai-rag/summary", ctrl.Summary)

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
		triggerReembedFn: func(ctx context.Context) error { called = true; return nil },
	}
	e, ctrl := newAdminAITestServer(mock)
	rec := servePostAdminAI(e, "/admin/ai-rag/reembed", `{}` , ctrl.Reembed)
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
		forceResyncFn: func(ctx context.Context) error { called = true; return nil },
	}
	e, ctrl := newAdminAITestServer(mock)
	rec := servePostAdminAI(e, "/admin/ai-rag/force-resync", `{}` , ctrl.ForceResync)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if !called {
		t.Errorf("expected ForceResync to be called")
	}
}
