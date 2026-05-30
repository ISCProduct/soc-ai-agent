package middleware

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

// ErrorResponse はAPIエラーレスポンスの統一形式
type ErrorResponse struct {
	Error  string `json:"error"`
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// APIError はエラーコードと詳細を持つ構造体。
// echo.NewHTTPError の Message として渡すことで構造化エラーを返せる。
type APIError struct {
	Code   string
	Msg    string
	Detail string
}

// defaultCodeByStatus は HTTPステータスコードからデフォルトのエラーコードを返す。
func defaultCodeByStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "VALIDATION_ERROR"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusUnprocessableEntity:
		return "VALIDATION_ERROR"
	case http.StatusTooManyRequests:
		return "TOO_MANY_REQUESTS"
	case http.StatusBadGateway:
		return "BAD_GATEWAY"
	case http.StatusServiceUnavailable:
		return "SERVICE_UNAVAILABLE"
	default:
		return "INTERNAL_ERROR"
	}
}

// CustomHTTPErrorHandler はEchoのグローバルエラーハンドラー。
// echo.HTTPError および一般的なerrorを統一JSON形式で返す。
func CustomHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	status := http.StatusInternalServerError
	resp := ErrorResponse{
		Error: http.StatusText(status),
		Code:  defaultCodeByStatus(status),
	}

	var he *echo.HTTPError
	if errors.As(err, &he) {
		status = he.Code
		switch msg := he.Message.(type) {
		case APIError:
			resp = ErrorResponse{
				Error:  msg.Msg,
				Code:   msg.Code,
				Detail: msg.Detail,
			}
		case string:
			resp = ErrorResponse{
				Error: msg,
				Code:  defaultCodeByStatus(status),
			}
		default:
			resp = ErrorResponse{
				Error: http.StatusText(status),
				Code:  defaultCodeByStatus(status),
			}
		}
	}

	_ = c.JSON(status, resp)
}
