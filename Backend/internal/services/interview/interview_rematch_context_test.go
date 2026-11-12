package interview

import (
	"context"
	"errors"
	"testing"

	"Backend/internal/services/costs"
	"Backend/internal/usagectx"
)

type exceededInterviewBudget struct{}

func (exceededInterviewBudget) Status(context.Context, uint) (costs.AIBudgetStatus, error) {
	return costs.AIBudgetStatus{Allowed: false}, nil
}

// 面接レポート後の再マッチングは、費用の配賦先と予算判定を載せた context で走る（#1776）。
func TestRematchContext(t *testing.T) {
	s := &InterviewService{}
	s.SetAIBudget(exceededInterviewBudget{})
	ctx := s.rematchContext(7, 3)
	if err := usagectx.CheckAIBudget(ctx); !errors.Is(err, usagectx.ErrAIBudgetExceeded) {
		t.Fatalf("CheckAIBudget = %v, want exceeded", err)
	}
	userID, orgID := usagectx.Actor(ctx)
	if userID == nil || *userID != 7 || orgID == nil || *orgID != 3 {
		t.Fatalf("actor = %v %v", userID, orgID)
	}

	// 予算サービスを配線していなければ判定は載らない（従来どおり）。
	if err := usagectx.CheckAIBudget((&InterviewService{}).rematchContext(7, 3)); err != nil {
		t.Fatalf("unwired: err = %v", err)
	}
}
