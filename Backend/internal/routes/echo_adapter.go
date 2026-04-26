package routes

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func wrapHTTPHandler(handler http.HandlerFunc) echo.HandlerFunc {
	return echo.WrapHandler(http.HandlerFunc(handler))
}
