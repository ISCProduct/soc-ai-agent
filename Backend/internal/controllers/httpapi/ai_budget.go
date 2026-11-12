package httpapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// AIBudgetExceededCode は予算超過の応答の code。フロントはこの値で超過を判別する。
const AIBudgetExceededCode = "ai_budget_exceeded"

// AIBudgetExceeded は予算超過の応答を返す。利用額は含めない
// （学校の利用額や上限は学生に見せない運用情報のため）。
func AIBudgetExceeded(c echo.Context) error {
	return c.JSON(http.StatusTooManyRequests, map[string]string{
		"error": "AIの利用上限に達しました。上限は日本時間の0時（月の上限は毎月1日の0時）に戻ります。",
		"code":  AIBudgetExceededCode,
	})
}

// AIBudgetUnavailable は予算を判定できないときのエラーを返す。
func AIBudgetUnavailable() error {
	return echo.NewHTTPError(http.StatusServiceUnavailable, "AIの利用上限を確認できません。時間をおいて再試行してください。")
}
