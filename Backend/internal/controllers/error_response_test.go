package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"

	"github.com/labstack/echo/v4"
)

// setupEcho はテスト用のEchoインスタンスを生成する
func setupEcho() *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomHTTPErrorHandler
	return e
}

func decodeErrResp(t *testing.T, rec *httptest.ResponseRecorder) middleware.ErrorResponse {
	t.Helper()
	var resp middleware.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("レスポンスのJSONデコード失敗: %v (body=%s)", err, rec.Body.String())
	}
	return resp
}

// ── newAPIError ────────────────────────────────────────────────────────────────

func TestNewAPIError_WithCode(t *testing.T) {
	e := setupEcho()
	e.GET("/", func(c echo.Context) error {
		return newAPIError(http.StatusConflict, ErrCodeDuplicateEmail, "email already exists")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	resp := decodeErrResp(t, rec)
	if resp.Code != ErrCodeDuplicateEmail {
		t.Errorf("code = %q, want %q", resp.Code, ErrCodeDuplicateEmail)
	}
	if resp.Error != "email already exists" {
		t.Errorf("error = %q, want %q", resp.Error, "email already exists")
	}
}

func TestNewAPIError_WithDetail(t *testing.T) {
	e := setupEcho()
	e.GET("/", func(c echo.Context) error {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "invalid input", "nameフィールドは必須です")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	resp := decodeErrResp(t, rec)
	if resp.Detail != "nameフィールドは必須です" {
		t.Errorf("detail = %q, want %q", resp.Detail, "nameフィールドは必須です")
	}
}

func TestNewAPIError_WithoutDetail(t *testing.T) {
	e := setupEcho()
	e.GET("/", func(c echo.Context) error {
		return newAPIError(http.StatusNotFound, ErrCodeNotFound, "not found")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("JSONデコード失敗: %v", err)
	}
	if _, exists := raw["detail"]; exists {
		t.Error("detail を省略したとき JSON に含まれるべきでない")
	}
}

// ── echoInternalError ─────────────────────────────────────────────────────────

func TestEchoInternalError(t *testing.T) {
	e := setupEcho()
	e.GET("/", func(c echo.Context) error {
		return echoInternalError(errors.New("db connection failed"))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	resp := decodeErrResp(t, rec)
	if resp.Code != ErrCodeInternalError {
		t.Errorf("code = %q, want %q", resp.Code, ErrCodeInternalError)
	}
	if resp.Error != internalServerErrorMessage {
		t.Errorf("error = %q, want %q", resp.Error, internalServerErrorMessage)
	}
}

// ── echoUintParam ─────────────────────────────────────────────────────────────

func TestEchoUintParam_Valid(t *testing.T) {
	e := setupEcho()
	e.GET("/items/:id", func(c echo.Context) error {
		id, err := echoUintParam(c, "id")
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, map[string]uint{"id": id})
	})

	req := httptest.NewRequest(http.MethodGet, "/items/42", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestEchoUintParam_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		param string
	}{
		{"文字列", "abc"},
		{"ゼロ", "0"},
		{"負の値", "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := setupEcho()
			e.GET("/items/:id", func(c echo.Context) error {
				_, err := echoUintParam(c, "id")
				return err
			})

			req := httptest.NewRequest(http.MethodGet, "/items/"+tt.param, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("param=%q: status = %d, want %d", tt.param, rec.Code, http.StatusBadRequest)
			}
			resp := decodeErrResp(t, rec)
			if resp.Code != ErrCodeValidationError {
				t.Errorf("param=%q: code = %q, want %q", tt.param, resp.Code, ErrCodeValidationError)
			}
		})
	}
}

// ── エラーコード定数 ──────────────────────────────────────────────────────────

func TestErrorCodeConstants(t *testing.T) {
	codes := []string{
		ErrCodeDuplicateEmail,
		ErrCodeNotFound,
		ErrCodeValidationError,
		ErrCodeUnauthorized,
		ErrCodeForbidden,
		ErrCodeInvalidStatus,
		ErrCodeInternalError,
		ErrCodeServiceUnavail,
		ErrCodeConflict,
		ErrCodeTooManyRequests,
	}
	seen := make(map[string]bool)
	for _, c := range codes {
		if c == "" {
			t.Error("空のエラーコード定数が存在する")
		}
		if seen[c] {
			t.Errorf("重複したエラーコード定数: %q", c)
		}
		seen[c] = true
	}
}
