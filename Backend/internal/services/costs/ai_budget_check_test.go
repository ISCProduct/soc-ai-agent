package costs

import (
	"context"
	"errors"
	"testing"

	"Backend/internal/usagectx"
)

type stubBudgetProvider struct {
	status AIBudgetStatus
	err    error
	calls  int
}

func (p *stubBudgetProvider) Status(context.Context, uint) (AIBudgetStatus, error) {
	p.calls++
	return p.status, p.err
}

func TestCheckUserAIBudget(t *testing.T) {
	tests := []struct {
		name     string
		provider AIBudgetStatusProvider
		want     error
	}{
		{name: "未設定は判定できない", provider: nil, want: usagectx.ErrAIBudgetUnavailable},
		{name: "読めなければ判定できない", provider: &stubBudgetProvider{err: errors.New("db down")}, want: usagectx.ErrAIBudgetUnavailable},
		{name: "上限に達していれば超過", provider: &stubBudgetProvider{status: AIBudgetStatus{Allowed: false}}, want: usagectx.ErrAIBudgetExceeded},
		{name: "枠内なら通す", provider: &stubBudgetProvider{status: AIBudgetStatus{Allowed: true}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckUserAIBudget(context.Background(), tt.provider, 1)
			if !errors.Is(err, tt.want) || (tt.want == nil && err != nil) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// バックグラウンドのワーカーは、載せた判定を usagectx.CheckAIBudget で実行できる。
func TestWithUserAIBudgetCheck(t *testing.T) {
	exceeded := &stubBudgetProvider{status: AIBudgetStatus{Allowed: false}}
	ctx := WithUserAIBudgetCheck(context.Background(), exceeded, 1)
	if err := usagectx.CheckAIBudget(ctx); !errors.Is(err, usagectx.ErrAIBudgetExceeded) {
		t.Fatalf("err = %v, want exceeded", err)
	}
	if exceeded.calls != 1 {
		t.Fatalf("calls = %d", exceeded.calls)
	}

	// 予算サービスを配線していない構成と、利用者のいないバッチには載せない。
	if err := usagectx.CheckAIBudget(WithUserAIBudgetCheck(context.Background(), nil, 1)); err != nil {
		t.Fatalf("nil provider: err = %v", err)
	}
	if err := usagectx.CheckAIBudget(WithUserAIBudgetCheck(context.Background(), exceeded, 0)); err != nil {
		t.Fatalf("user 0: err = %v", err)
	}
}
