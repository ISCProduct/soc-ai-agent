package controllers

import (
	"log"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"

	"Backend/internal/middleware"

	"github.com/labstack/echo/v4"
)

// echoUintParam はパスパラメータを uint として取得する。
func echoUintParam(c echo.Context, key string) (uint, error) {
	s := c.Param(key)
	id, err := strconv.ParseUint(s, 10, 64)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "invalid "+key)
	}
	return uint(id), nil
}

const internalServerErrorMessage = "内部エラーが発生しました"

// echoInternalError はエラーをログ出力しつつ echo.HTTPError を返す。
func echoInternalError(err error) error {
	logError(err)
	return echo.NewHTTPError(http.StatusInternalServerError, internalServerErrorMessage)
}

// echoUserID は echo.Context のリクエストコンテキストからユーザーIDを取得する。
func echoUserID(c echo.Context) (uint, bool) {
	userID, ok := c.Request().Context().Value(middleware.UserIDContextKey).(uint)
	return userID, ok && userID != 0
}

// echoIntQuery はクエリパラメータを整数として取得し、取得できない場合はデフォルト値を返す。
func echoIntQuery(c echo.Context, key string, def int) int {
	v := c.QueryParam(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func logError(err error) {
	if err == nil {
		return
	}
	if _, file, line, ok := runtime.Caller(2); ok {
		log.Printf("[ERROR] %s:%d %v", filepath.Base(file), line, err)
	} else {
		log.Printf("[ERROR] %v", err)
	}
}
