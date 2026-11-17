package chat_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	chatcontrollers "Backend/internal/controllers/chat"
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/services/matching"
	"Backend/internal/services/shared"

	"github.com/labstack/echo/v4"
)

// SOCAIAGENT-611: GET /api/chat/matches/:id/axes

func axesCtx(id string, userID uint) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodGet, "/api/chat/matches/"+id+"/axes", nil)
	if userID > 0 {
		req = testsupport.WithUserID(req, userID)
	}
	rec := httptest.NewRecorder()
	ctx := testsupport.NewCtx(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues(id)
	return ctx, rec
}

func TestChatController_GetAxisBreakdown(t *testing.T) {
	matchSvc := &mocks.MatchingServiceMock{}
	matchSvc.On("GetAxisBreakdown", uint(5), uint(1)).Return(&matching.AxisBreakdown{MatchID: 5}, nil)
	ctx, rec := axesCtx("5", 1)
	if err := newChatController(nil, matchSvc, nil, nil, nil).GetAxisBreakdown(ctx); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("err = %v code = %d", err, rec.Code)
	}
	matchSvc.AssertExpectations(t)
}

func TestChatController_GetAxisBreakdown_Errors(t *testing.T) {
	c := chatcontrollers.NewChatController(nil, nil, nil, nil, nil)
	ctx, _ := axesCtx("5", 0)
	testsupport.AssertStatus(t, c.GetAxisBreakdown, ctx, http.StatusUnauthorized)
	for _, id := range []string{"0", "abc", "-1"} {
		ctx, _ := axesCtx(id, 1)
		testsupport.AssertStatus(t, c.GetAxisBreakdown, ctx, http.StatusBadRequest)
	}

	// 他人の推薦・存在しない推薦は 404（service が shared.ErrNotFound を返す）
	notFound := &mocks.MatchingServiceMock{}
	notFound.On("GetAxisBreakdown", uint(5), uint(1)).Return(nil, shared.ErrNotFound)
	ctx, _ = axesCtx("5", 1)
	testsupport.AssertStatus(t, newChatController(nil, notFound, nil, nil, nil).GetAxisBreakdown, ctx, http.StatusNotFound)

	failing := &mocks.MatchingServiceMock{}
	failing.On("GetAxisBreakdown", uint(5), uint(1)).Return(nil, errors.New("db"))
	ctx, _ = axesCtx("5", 1)
	testsupport.AssertStatus(t, newChatController(nil, failing, nil, nil, nil).GetAxisBreakdown, ctx, http.StatusInternalServerError)
}
