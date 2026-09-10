package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// EchoRequestLogger はリクエストの method / path / status / duration を
// 構造化ログで記録する。X-Request-ID が存在する場合はフィールドに含める。
//
// echo.WrapMiddleware で http.Handler 版を包む形にはしない。ハンドラがエラーを
// 返した時点ではまだ HTTPErrorHandler がレスポンスを書いておらず、status を
// ResponseWriter から取ると初期値の 200 のまま記録されるため。401 や 500 が
// すべて 200 として残ると、ログを見ても失敗に気づけない。
func EchoRequestLogger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()
		err := next(c)

		// エラーを返した場合、レスポンスを書くのはこの後に走る
		// CustomHTTPErrorHandler なので、そちらと同じ判定でステータスを決める。
		status := c.Response().Status
		if err != nil {
			status = statusFromError(err)
		}

		attrs := []any{
			slog.String("method", c.Request().Method),
			slog.String("path", c.Request().URL.Path),
			slog.Int("status", status),
			slog.Duration("duration", time.Since(start)),
		}
		if rid := GetRequestID(c.Request().Context()); rid != "" {
			attrs = append(attrs, slog.String("request_id", rid))
		}

		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}
		slog.Log(c.Request().Context(), level, "http request", attrs...)

		return err
	}
}

// statusFromError は CustomHTTPErrorHandler が返すのと同じステータスを求める。
func statusFromError(err error) int {
	var he *echo.HTTPError
	if errors.As(err, &he) {
		return he.Code
	}
	return http.StatusInternalServerError
}
