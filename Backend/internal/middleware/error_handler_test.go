package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func newTestEcho() *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = CustomHTTPErrorHandler
	return e
}

func decodeErrorResponse(t *testing.T, rec *httptest.ResponseRecorder) ErrorResponse {
	t.Helper()
	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("レスポンスのJSONデコード失敗: %v (body=%s)", err, rec.Body.String())
	}
	return resp
}

// TestDefaultCodeByStatus はHTTPステータスコードからエラーコードへの変換を検証する
func TestDefaultCodeByStatus(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{http.StatusBadRequest, "VALIDATION_ERROR"},
		{http.StatusUnauthorized, "UNAUTHORIZED"},
		{http.StatusForbidden, "FORBIDDEN"},
		{http.StatusNotFound, "NOT_FOUND"},
		{http.StatusConflict, "CONFLICT"},
		{http.StatusUnprocessableEntity, "VALIDATION_ERROR"},
		{http.StatusTooManyRequests, "TOO_MANY_REQUESTS"},
		{http.StatusBadGateway, "BAD_GATEWAY"},
		{http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE"},
		{http.StatusInternalServerError, "INTERNAL_ERROR"},
		{http.StatusTeapot, "INTERNAL_ERROR"}, // 未定義ステータスはINTERNAL_ERRORにフォールバック
	}

	for _, tt := range tests {
		got := defaultCodeByStatus(tt.status)
		if got != tt.want {
			t.Errorf("defaultCodeByStatus(%d) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

// TestCustomHTTPErrorHandler_PlainStringMessage は文字列メッセージのecho.HTTPErrorを検証する
func TestCustomHTTPErrorHandler_PlainStringMessage(t *testing.T) {
	e := newTestEcho()
	e.GET("/test", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid input")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	resp := decodeErrorResponse(t, rec)
	if resp.Error != "invalid input" {
		t.Errorf("error = %q, want %q", resp.Error, "invalid input")
	}
	if resp.Code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want %q", resp.Code, "VALIDATION_ERROR")
	}
	if resp.Detail != "" {
		t.Errorf("detail は空であるべき, got %q", resp.Detail)
	}
}

// TestCustomHTTPErrorHandler_APIErrorMessage はAPIError構造体を使ったエラーを検証する
func TestCustomHTTPErrorHandler_APIErrorMessage(t *testing.T) {
	e := newTestEcho()
	e.GET("/test", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusConflict, APIError{
			Code:   "DUPLICATE_EMAIL",
			Msg:    "email already exists",
			Detail: "このメールアドレスは既に登録されています",
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	resp := decodeErrorResponse(t, rec)
	if resp.Error != "email already exists" {
		t.Errorf("error = %q, want %q", resp.Error, "email already exists")
	}
	if resp.Code != "DUPLICATE_EMAIL" {
		t.Errorf("code = %q, want %q", resp.Code, "DUPLICATE_EMAIL")
	}
	if resp.Detail != "このメールアドレスは既に登録されています" {
		t.Errorf("detail = %q, want %q", resp.Detail, "このメールアドレスは既に登録されています")
	}
}

// TestCustomHTTPErrorHandler_APIErrorWithoutDetail はDetailが空のAPIErrorを検証する
func TestCustomHTTPErrorHandler_APIErrorWithoutDetail(t *testing.T) {
	e := newTestEcho()
	e.GET("/test", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, APIError{
			Code: "NOT_FOUND",
			Msg:  "user not found",
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	resp := decodeErrorResponse(t, rec)
	if resp.Code != "NOT_FOUND" {
		t.Errorf("code = %q, want %q", resp.Code, "NOT_FOUND")
	}

	// detail フィールドがJSONに含まれないことを確認
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("JSONデコード失敗: %v", err)
	}
	if _, exists := raw["detail"]; exists {
		t.Error("detail が空のとき JSON に含まれるべきでない")
	}
}

// TestCustomHTTPErrorHandler_InternalServerError は500エラーを検証する
func TestCustomHTTPErrorHandler_InternalServerError(t *testing.T) {
	e := newTestEcho()
	e.GET("/test", func(c echo.Context) error {
		return errors.New("unexpected db error")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	resp := decodeErrorResponse(t, rec)
	if resp.Code != "INTERNAL_ERROR" {
		t.Errorf("code = %q, want %q", resp.Code, "INTERNAL_ERROR")
	}
}

// TestCustomHTTPErrorHandler_ResponseAlreadyCommitted はレスポンス送信済みの場合に二重送信しないことを検証する
func TestCustomHTTPErrorHandler_ResponseAlreadyCommitted(t *testing.T) {
	e := newTestEcho()
	e.GET("/test", func(c echo.Context) error {
		// 先に200レスポンスを送信してCommittedをtrueにする
		_ = c.String(http.StatusOK, "ok")
		return echo.NewHTTPError(http.StatusBadRequest, "too late")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// 最初のレスポンス（200）が返り、エラーで上書きされないこと
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestErrorResponse_JSONSerialization はErrorResponseのJSONシリアライズを検証する
func TestErrorResponse_JSONSerialization(t *testing.T) {
	tests := []struct {
		name       string
		resp       ErrorResponse
		wantDetail bool
	}{
		{
			name:       "detail あり",
			resp:       ErrorResponse{Error: "msg", Code: "CODE", Detail: "detail"},
			wantDetail: true,
		},
		{
			name:       "detail なし（omitempty）",
			resp:       ErrorResponse{Error: "msg", Code: "CODE"},
			wantDetail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.resp)
			if err != nil {
				t.Fatalf("JSONマーシャル失敗: %v", err)
			}
			var raw map[string]any
			if err := json.Unmarshal(b, &raw); err != nil {
				t.Fatalf("JSONアンマーシャル失敗: %v", err)
			}
			_, hasDetail := raw["detail"]
			if hasDetail != tt.wantDetail {
				t.Errorf("detail 存在 = %v, want %v", hasDetail, tt.wantDetail)
			}
		})
	}
}
