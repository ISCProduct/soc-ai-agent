package controllers_test

// AdminCostsControllerのHTTPハンドラーテスト (Issue #430)
//
// 実行: cd Backend && go test ./test/controllers/... -run "AdminCosts" -v

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	"Backend/internal/services"
	ifaces "Backend/internal/services/interfaces"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newAdminCostsController(cost ifaces.APICostService, realtime ifaces.RealtimeUsageService) *controllers.AdminCostsController {
	return controllers.NewAdminCostsController(cost, realtime)
}

// ===== Summary =====

func TestAdminCostsController_Summary_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/costs/summary", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminCostsController(nil, nil).Summary(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminCostsController_Summary_CostServiceError(t *testing.T) {
	cost := &mocks.APICostServiceMock{}
	cost.On("GetCurrentMonthTotal").Return(0.0, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/costs/summary", nil)
	w := httptest.NewRecorder()
	newAdminCostsController(cost, nil).Summary(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	cost.AssertExpectations(t)
}

func TestAdminCostsController_Summary_ModelBreakdownError(t *testing.T) {
	cost := &mocks.APICostServiceMock{}
	cost.On("GetCurrentMonthTotal").Return(1.5, nil)
	cost.On("GetModelBreakdown", mock.Anything).Return(nil, errors.New("breakdown error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/costs/summary", nil)
	w := httptest.NewRecorder()
	newAdminCostsController(cost, nil).Summary(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	cost.AssertExpectations(t)
}

func TestAdminCostsController_Summary_Success_NoRealtime(t *testing.T) {
	cost := &mocks.APICostServiceMock{}
	cost.On("GetCurrentMonthTotal").Return(1.5, nil)
	cost.On("GetModelBreakdown", mock.Anything).Return([]services.ModelCostSummary{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/costs/summary", nil)
	w := httptest.NewRecorder()
	newAdminCostsController(cost, nil).Summary(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	cost.AssertExpectations(t)
}

func TestAdminCostsController_Summary_Success_WithRealtime(t *testing.T) {
	cost := &mocks.APICostServiceMock{}
	realtime := &mocks.RealtimeUsageServiceMock{}
	cost.On("GetCurrentMonthTotal").Return(1.5, nil)
	cost.On("GetModelBreakdown", mock.Anything).Return([]services.ModelCostSummary{}, nil)
	realtime.On("CurrentMonthTotalCost").Return(0.5, nil)
	realtime.On("CurrentActiveCount").Return(int64(3), nil)
	realtime.On("GetUserBreakdown", 30, 20).Return([]services.RealtimeUserSummary{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/costs/summary", nil)
	w := httptest.NewRecorder()
	newAdminCostsController(cost, realtime).Summary(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	cost.AssertExpectations(t)
	realtime.AssertExpectations(t)
}

// ===== Daily =====

func TestAdminCostsController_Daily_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/costs/daily", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminCostsController(nil, nil).Daily(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminCostsController_Daily_ServiceError(t *testing.T) {
	cost := &mocks.APICostServiceMock{}
	cost.On("GetDailyCosts", 30).Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/costs/daily", nil)
	w := httptest.NewRecorder()
	newAdminCostsController(cost, nil).Daily(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	cost.AssertExpectations(t)
}

func TestAdminCostsController_Daily_Success_NoRealtime(t *testing.T) {
	cost := &mocks.APICostServiceMock{}
	cost.On("GetDailyCosts", 30).Return([]services.DailyCostSummary{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/costs/daily", nil)
	w := httptest.NewRecorder()
	newAdminCostsController(cost, nil).Daily(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	cost.AssertExpectations(t)
}

func TestAdminCostsController_Daily_Success_WithRealtime(t *testing.T) {
	cost := &mocks.APICostServiceMock{}
	realtime := &mocks.RealtimeUsageServiceMock{}
	cost.On("GetDailyCosts", 30).Return([]services.DailyCostSummary{}, nil)
	realtime.On("GetDailyUsage", 30).Return([]services.RealtimeDailySummary{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/costs/daily", nil)
	w := httptest.NewRecorder()
	newAdminCostsController(cost, realtime).Daily(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ===== Monthly =====

func TestAdminCostsController_Monthly_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/costs/monthly", nil)
	w := httptest.NewRecorder()
	controllers.NewAdminCostsController(nil, nil).Monthly(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAdminCostsController_Monthly_ServiceError(t *testing.T) {
	cost := &mocks.APICostServiceMock{}
	cost.On("GetMonthlyCosts", 12).Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/costs/monthly", nil)
	w := httptest.NewRecorder()
	newAdminCostsController(cost, nil).Monthly(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	cost.AssertExpectations(t)
}

func TestAdminCostsController_Monthly_Success_NoRealtime(t *testing.T) {
	cost := &mocks.APICostServiceMock{}
	cost.On("GetMonthlyCosts", 12).Return([]services.MonthlyCostSummary{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/costs/monthly", nil)
	w := httptest.NewRecorder()
	newAdminCostsController(cost, nil).Monthly(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	cost.AssertExpectations(t)
}

func TestAdminCostsController_Monthly_Success_WithRealtime(t *testing.T) {
	cost := &mocks.APICostServiceMock{}
	realtime := &mocks.RealtimeUsageServiceMock{}
	cost.On("GetMonthlyCosts", 12).Return([]services.MonthlyCostSummary{}, nil)
	realtime.On("GetMonthlyUsage", 12).Return([]services.RealtimeMonthlySummary{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/costs/monthly", nil)
	w := httptest.NewRecorder()
	newAdminCostsController(cost, realtime).Monthly(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
