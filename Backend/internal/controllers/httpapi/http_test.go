package httpapi

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

// ── NewAPIError ────────────────────────────────────────────────────────────────

func TestNewAPIError_WithCode(t *testing.T) {
	e := setupEcho()
	e.GET("/", func(c echo.Context) error {
		return NewAPIError(http.StatusConflict, ErrCodeDuplicateEmail, "email already exists")
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
		return NewAPIError(http.StatusBadRequest, ErrCodeValidationError, "invalid input", "nameフィールドは必須です")
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
		return NewAPIError(http.StatusNotFound, ErrCodeNotFound, "not found")
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

// ── InternalError ─────────────────────────────────────────────────────────

func TestEchoInternalError(t *testing.T) {
	e := setupEcho()
	e.GET("/", func(c echo.Context) error {
		return InternalError(errors.New("db connection failed"))
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
	if resp.Error != InternalServerErrorMessage {
		t.Errorf("error = %q, want %q", resp.Error, InternalServerErrorMessage)
	}
}

// ── UintParam ─────────────────────────────────────────────────────────────

func TestEchoUintParam_Valid(t *testing.T) {
	e := setupEcho()
	e.GET("/items/:id", func(c echo.Context) error {
		id, err := UintParam(c, "id")
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
				_, err := UintParam(c, "id")
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

// ── LimitQuery ────────────────────────────────────────────────────────────

// TestLimitQuery は一覧APIの limit が既定値と上限100の間に収まることを固定する(#1478)。
//
// 上限が無いと limit=1000000 がそのままサービス/DBへ渡り、1リクエストで
// 全件が読まれてJSON化される。
func TestLimitQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		def   int
		want  int
	}{
		{name: "未指定は既定値", query: "", def: 20, want: 20},
		{name: "上限内はそのまま", query: "?limit=30", def: 20, want: 30},
		{name: "上限ちょうど", query: "?limit=100", def: 20, want: 100},
		{name: "上限超過は100へ頭打ち", query: "?limit=101", def: 20, want: MaxListLimit},
		{name: "極端な値も100へ頭打ち", query: "?limit=1000000", def: 20, want: MaxListLimit},
		{name: "0は既定値", query: "?limit=0", def: 10, want: 10},
		{name: "負値は既定値", query: "?limit=-1", def: 10, want: 10},
		{name: "数値以外は既定値", query: "?limit=abc", def: 10, want: 10},
		{name: "既定値が上限を超えていても100へ頭打ち", query: "", def: 500, want: MaxListLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+tt.query, nil)
			rec := httptest.NewRecorder()
			c := setupEcho().NewContext(req, rec)
			if got := LimitQuery(c, "limit", tt.def); got != tt.want {
				t.Fatalf("LimitQuery() = %d, want %d", got, tt.want)
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
