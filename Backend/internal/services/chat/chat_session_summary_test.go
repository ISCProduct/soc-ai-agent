package chat

import (
	"context"
	"testing"
)

func TestSessionSummaryForResponse_MissingDepsReturnsNil(t *testing.T) {
	s := &ChatService{}
	if got := s.sessionSummaryForResponse(context.Background(), 1, "s1"); got != nil {
		t.Fatalf("got %#v, want nil", got)
	}
}
