package middleware

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

// ErrorResponse はAPIエラーレスポンスの統一形式
type ErrorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

// CustomHTTPErrorHandler はEchoのグローバルエラーハンドラー。
// echo.HTTPError および一般的なerrorを統一JSON形式で返す。
func CustomHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	code := http.StatusInternalServerError
	message := http.StatusText(code)

	var he *echo.HTTPError
	if errors.As(err, &he) {
		code = he.Code
		if m, ok := he.Message.(string); ok {
			message = m
		}
	}

	_ = c.JSON(code, ErrorResponse{Error: message, Code: code})
}
