package routes

import (
	"Backend/internal/controllers/httpapi"
	"Backend/internal/services/costs"
	"Backend/internal/usagectx"
	"context"
	"errors"

	"github.com/labstack/echo/v4"
)

type aiBudgetStatusProvider = costs.AIBudgetStatusProvider

// AIBudgetGuards はAIを実行するルートに付ける予算判定のミドルウェア。
// 認証ミドルウェアの後段に付ける。失効トークンや退会済みの利用者に判定の費用を使わないため。
//
// 判定は過去の利用額を読むだけで、ロックも予約もしない。同時に走ったリクエストのぶんだけ
// 上限を少し超えることを許す。ロックを持つと、AI の応答待ちの間 DB 接続を占有し、
// 同じ学校の利用者を直列化してしまう（#1776）。
type AIBudgetGuards struct {
	// Immediate はハンドラの前に判定する。常に AI を呼ぶルートに使う。
	Immediate echo.MiddlewareFunc
	// Deferred は判定を AI を呼ぶ直前まで遅らせる。DB やキャッシュで返せることがあるルートに使う。
	// 上限に達した利用者でも、費用のかからない結果は受け取れるようにするため。
	Deferred echo.MiddlewareFunc
}

// NewAIBudgetGuards は予算判定のミドルウェアを作る。
func NewAIBudgetGuards(service aiBudgetStatusProvider) AIBudgetGuards {
	return AIBudgetGuards{Immediate: echoAIBudget(service), Deferred: echoDeferredAIBudget(service)}
}

// echoAIBudget はハンドラの前に予算を判定する。
// 未ログインの利用者は予算の対象ではなく、ゲスト向けのレート制限で保護する。
func echoAIBudget(service aiBudgetStatusProvider) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, ok := httpapi.UserID(c)
			if !ok {
				return next(c)
			}
			err := checkAIBudget(c.Request().Context(), service, userID)
			switch {
			case err == nil:
				return next(c)
			case isAIBudgetExceeded(err):
				return httpapi.AIBudgetExceeded(c)
			default:
				httpapi.LogError(err)
				return httpapi.AIBudgetUnavailable()
			}
		}
	}
}

// echoDeferredAIBudget は判定を context に載せ、AI を呼ぶ直前に usagectx.CheckAIBudget で実行させる。
func echoDeferredAIBudget(service aiBudgetStatusProvider) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, ok := httpapi.UserID(c)
			if !ok {
				return next(c)
			}
			ctx := usagectx.WithAIBudgetCheck(c.Request().Context(), func(ctx context.Context) error {
				return checkAIBudget(ctx, service, userID)
			})
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// checkAIBudget はバックグラウンドの経路と同じ判定を使う（costs.CheckUserAIBudget）。
func checkAIBudget(ctx context.Context, service aiBudgetStatusProvider, userID uint) error {
	return costs.CheckUserAIBudget(ctx, service, userID)
}

func isAIBudgetExceeded(err error) bool {
	return errors.Is(err, usagectx.ErrAIBudgetExceeded)
}
