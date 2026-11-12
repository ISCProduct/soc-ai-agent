package costs

import (
	"context"
	"fmt"

	"Backend/internal/usagectx"
)

// AIBudgetStatusProvider は利用者の予算の状態を返す。*AIBudgetService が満たす。
type AIBudgetStatusProvider interface {
	Status(context.Context, uint) (AIBudgetStatus, error)
}

// CheckUserAIBudget は利用者が新しい AI を使えるかを判定する。
// 上限に達していれば usagectx.ErrAIBudgetExceeded、判定できなければ
// usagectx.ErrAIBudgetUnavailable を包んだエラーを返す。
func CheckUserAIBudget(ctx context.Context, provider AIBudgetStatusProvider, userID uint) error {
	if provider == nil {
		return fmt.Errorf("%w: service is not configured", usagectx.ErrAIBudgetUnavailable)
	}
	status, err := provider.Status(ctx, userID)
	if err != nil {
		return fmt.Errorf("%w: %v", usagectx.ErrAIBudgetUnavailable, err)
	}
	if !status.Allowed {
		return usagectx.ErrAIBudgetExceeded
	}
	return nil
}

// WithUserAIBudgetCheck は利用者の予算判定を context に載せる。
//
// HTTP の経路は認証の後段のミドルウェアが載せる。リクエストから切り離したワーカー
// （context.Background() から始めるチャット後・面接レポート後の再マッチングなど）には
// 届かないので、ワーカーを始めるときにこれを呼ぶ。載せないと usagectx.CheckAIBudget が
// 何もせず、上限を超えた利用者にも AI の費用がかかる。
//
// provider が nil か userID が 0 なら何も載せない（予算サービスを配線していない構成と、
// 利用者のいないバッチ）。
func WithUserAIBudgetCheck(ctx context.Context, provider AIBudgetStatusProvider, userID uint) context.Context {
	if provider == nil || userID == 0 {
		return ctx
	}
	return usagectx.WithAIBudgetCheck(ctx, func(ctx context.Context) error {
		return CheckUserAIBudget(ctx, provider, userID)
	})
}
