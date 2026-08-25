package controllers

import (
	"log"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"

	"Backend/internal/logger"
	"Backend/internal/middleware"
	"Backend/internal/services"

	"github.com/labstack/echo/v4"
)

// ── エラーコード定数 ──────────────────────────────────────────────────────────

const (
	ErrCodeDuplicateEmail  = "DUPLICATE_EMAIL"
	ErrCodeNotFound        = "NOT_FOUND"
	ErrCodeValidationError = "VALIDATION_ERROR"
	ErrCodeUnauthorized    = "UNAUTHORIZED"
	ErrCodeForbidden       = "FORBIDDEN"
	ErrCodeInvalidStatus   = "INVALID_STATUS"
	ErrCodeInternalError   = "INTERNAL_ERROR"
	ErrCodeServiceUnavail  = "SERVICE_UNAVAILABLE"
	ErrCodeConflict        = "CONFLICT"
	ErrCodeTooManyRequests = "TOO_MANY_REQUESTS"
)

// ── ヘルパー関数 ──────────────────────────────────────────────────────────────

// newAPIError はエラーコード付きの echo エラーを生成する。
// detail は省略可能（省略時はレスポンスに含まれない）。
func newAPIError(status int, code, message string, detail ...string) error {
	d := ""
	if len(detail) > 0 {
		d = detail[0]
	}
	return echo.NewHTTPError(status, middleware.APIError{
		Code:   code,
		Msg:    message,
		Detail: d,
	})
}

// echoUintParam はパスパラメータを uint として取得する。
func echoUintParam(c echo.Context, key string) (uint, error) {
	s := c.Param(key)
	id, err := strconv.ParseUint(s, 10, 64)
	if err != nil || id == 0 {
		return 0, newAPIError(http.StatusBadRequest, ErrCodeValidationError, "invalid "+key)
	}
	return uint(id), nil
}

const internalServerErrorMessage = "内部エラーが発生しました"

// echoInternalError はエラーをログ出力しつつ echo.HTTPError を返す。
func echoInternalError(err error) error {
	logError(err)
	return newAPIError(http.StatusInternalServerError, ErrCodeInternalError, internalServerErrorMessage)
}

// echoUserID は echo.Context のリクエストコンテキストからユーザーIDを取得する。
func echoUserID(c echo.Context) (uint, bool) {
	userID, ok := c.Request().Context().Value(middleware.UserIDContextKey).(uint)
	return userID, ok && userID != 0
}

// ensureAdminSchoolAccess は、対象リソースの学校ID(未割当ならnil)に対して、
// 呼び出し元admin(担当校制限がある場合)がアクセスしてよいかを検証する共通ヘルパー
// (#980/#981/#982/#984で同一ロジックが3コントローラーに重複していたのを統合)。
// schoolsが未設定(呼び出し元でのDI漏れ)の場合はfail-closedで拒否する。
func ensureAdminSchoolAccess(ctx echo.Context, schools *services.SchoolService, targetSchoolID *uint) error {
	if schools == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "school access check is not configured")
	}
	adminUserID, ok := middleware.AdminUserIDFromContext(ctx.Request().Context())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	allowed, err := schools.CanAdminAccessSchool(adminUserID, targetSchoolID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to resolve school access")
	}
	if !allowed {
		return echo.NewHTTPError(http.StatusForbidden, "school access denied")
	}
	return nil
}

// echoRequiredUintQuery は必須の正の整数クエリパラメータを返す。欠落・不正は 400。
func echoRequiredUintQuery(c echo.Context, key string) (uint, error) {
	raw := c.QueryParam(key)
	if raw == "" {
		return 0, echo.NewHTTPError(http.StatusBadRequest, key+" is required")
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "invalid "+key)
	}
	return uint(id), nil
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

// Exported wrappers for testing from external packages.

const InternalServerErrorMessage = internalServerErrorMessage

func NewAPIError(status int, code, message string, detail ...string) error {
	return newAPIError(status, code, message, detail...)
}
func EchoUintParam(c echo.Context, key string) (uint, error) { return echoUintParam(c, key) }
func EchoInternalError(err error) error                      { return echoInternalError(err) }

func logError(err error) {
	if err == nil {
		return
	}
	file, line := "", 0
	if _, f, l, ok := runtime.Caller(2); ok {
		file, line = filepath.Base(f), l
		log.Printf("[ERROR] %s:%d %v", file, line, err)
	} else {
		log.Printf("[ERROR] %v", err)
	}
	logger.PutErrorJSON("internal", map[string]any{
		"error": err.Error(),
		"file":  file,
		"line":  line,
	})
}
