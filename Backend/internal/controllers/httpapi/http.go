package httpapi

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"

	"Backend/internal/logger"
	"Backend/internal/middleware"
	"Backend/internal/openai"
	"Backend/internal/services/school"

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

// NewAPIError はエラーコード付きの echo エラーを生成する。
// detail は省略可能（省略時はレスポンスに含まれない）。
func NewAPIError(status int, code, message string, detail ...string) error {
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

// UintParam はパスパラメータを uint として取得する。
func UintParam(c echo.Context, key string) (uint, error) {
	s := c.Param(key)
	id, err := strconv.ParseUint(s, 10, 64)
	if err != nil || id == 0 {
		return 0, NewAPIError(http.StatusBadRequest, ErrCodeValidationError, "invalid "+key)
	}
	return uint(id), nil
}

const InternalServerErrorMessage = "内部エラーが発生しました"

// InternalError はエラーをログ出力しつつ echo.HTTPError を返す。
func InternalError(err error) error {
	LogError(err)
	// AI プロバイダ未設定・ローカル推論先の障害は設定/運用の問題で、リトライすれば
	// 回復しうる。500 + 「内部エラー」だと API 利用者が原因を切り分けられないため
	// 503 + 明示メッセージにする(#1293)。InternalError を通る全経路に一様に効かせる。
	if errors.Is(err, openai.ErrAIUnavailable) {
		return NewAPIError(http.StatusServiceUnavailable, ErrCodeServiceUnavail,
			"現在AI機能を利用できません。しばらくしてから再度お試しください。")
	}
	return NewAPIError(http.StatusInternalServerError, ErrCodeInternalError, InternalServerErrorMessage)
}

// UserID は echo.Context のリクエストコンテキストからユーザーIDを取得する。
func UserID(c echo.Context) (uint, bool) {
	userID, ok := c.Request().Context().Value(middleware.UserIDContextKey).(uint)
	return userID, ok && userID != 0
}

// CompanyUserID は企業ポータル認証済みユーザーIDを取得する。
func CompanyUserID(c echo.Context) (uint, bool) {
	companyUserID, ok := middleware.CompanyUserIDFromContext(c.Request().Context())
	return companyUserID, ok
}

// EnsureAdminSchoolAccess は、対象リソースの学校ID(未割当ならnil)に対して、
// 呼び出し元admin(担当校制限がある場合)がアクセスしてよいかを検証する共通ヘルパー
// (#980/#981/#982/#984で同一ロジックが3コントローラーに重複していたのを統合)。
// schoolsが未設定(呼び出し元でのDI漏れ)の場合はfail-closedで拒否する。
func EnsureAdminSchoolAccess(ctx echo.Context, schools *school.SchoolService, targetSchoolID *uint) error {
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

// AdminSchoolFilter は EchoAdminSchoolScope が設定した絞り込み対象(nilは絞り込みなし)を返す。
//
// ミドルウェア未適用のルートから呼ばれた場合は fail-closed で 500 を返す(#1157)。
// `schoolID, _ :=` で第2戻り値を捨てると、ルートから schoolScope が外れたときに
// 「絞り込みなし」と区別できず、全校のデータが返る fail-open になる。
func AdminSchoolFilter(ctx echo.Context) (*uint, error) {
	filter, ok := middleware.AdminSchoolFilterFromContext(ctx.Request().Context())
	if !ok {
		// 既存の teacher insight 経路と同じ 403 に揃える（#1027 の前例）。
		// ただし 403 だけでは「正当な権限拒否」と区別できず、ルート定義から
		// schoolScope が外れた設定ミスに気づけないのでログを残す(#1157)。
		LogError(fmt.Errorf("school scope middleware is missing: %s %s",
			ctx.Request().Method, ctx.Request().URL.Path))
		return nil, echo.NewHTTPError(http.StatusForbidden, "school scope is not resolved")
	}
	return filter, nil
}

// RequiredUintQuery は必須の正の整数クエリパラメータを返す。欠落・不正は 400。
func RequiredUintQuery(c echo.Context, key string) (uint, error) {
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

// IntQuery はクエリパラメータを整数として取得し、取得できない場合はデフォルト値を返す。
func IntQuery(c echo.Context, key string, def int) int {
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

// MaxListLimit は一覧APIの1リクエストあたり取得件数の上限。
// admin の user/school/organization/company 系が 100 で頭打ちにしているのに揃える(#1412)。
const MaxListLimit = 100

// LimitQuery は一覧APIの limit クエリを取得し、MaxListLimit で頭打ちにする。
// 未指定・不正値・0以下は def を返す（def 自体も上限を超えない）。
//
// 上限が無いと limit=1000000 がそのままサービス/DBへ渡り、1リクエストで
// 全件が読まれてJSON化される(#1478)。同じ定数が各コントローラーに散らばると
// 追加したエンドポイントで付け忘れるため、ここに集約する。
func LimitQuery(c echo.Context, key string, def int) int {
	return min(IntQuery(c, key, def), MaxListLimit)
}

func LogError(err error) {
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
