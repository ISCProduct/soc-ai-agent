package companyfetch_test

import (
	"Backend/internal/companyfetch"
	"context"
	"errors"
	"testing"
)

type denyBudget struct{}

func (denyBudget) AllowSearch() error { return companyfetch.ErrSearchBudgetExceeded }

type allowBudget struct{ calls int }

func (b *allowBudget) AllowSearch() error {
	b.calls++
	return nil
}

func TestSearchLiteJSON_BudgetExceeded(t *testing.T) {
	llm := &companyfetch.LLM{Budget: denyBudget{}}
	_, _, err := llm.SearchLiteJSON(context.Background(), "prompt", 100)
	if !errors.Is(err, companyfetch.ErrSearchBudgetExceeded) {
		t.Fatalf("want ErrSearchBudgetExceeded, got %v", err)
	}
}

func TestSearchLiteJSON_NilClientStillErrorsAfterBudget(t *testing.T) {
	b := &allowBudget{}
	llm := &companyfetch.LLM{Budget: b}
	_, _, err := llm.SearchLiteJSON(context.Background(), "prompt", 100)
	if err == nil {
		t.Fatal("expected nil client error")
	}
	if b.calls != 1 {
		t.Fatalf("budget AllowSearch calls=%d want 1", b.calls)
	}
}
