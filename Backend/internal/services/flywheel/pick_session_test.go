package flywheel

import (
	"errors"
	"testing"
)

func TestPickDiagnosisSessionID(t *testing.T) {
	got := PickDiagnosisSessionID(8, "chat-abc", nil)
	if got != "chat-abc" {
		t.Fatalf("got %q", got)
	}

	got = PickDiagnosisSessionID(8, "", errors.New("missing"))
	if got != "interview-8" {
		t.Fatalf("fallback got %q", got)
	}

	got = PickDiagnosisSessionID(8, "interview-8", nil)
	if got != "interview-8" {
		t.Fatalf("interview-only got %q", got)
	}
}
