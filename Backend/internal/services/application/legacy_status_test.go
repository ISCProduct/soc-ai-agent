package application

import "testing"

func TestNormalizeLegacyStatus(t *testing.T) {
	if got := normalizeLegacyStatus("interview"); got != "interview_in_progress" {
		t.Fatalf("interview -> %s", got)
	}
	if got := normalizeLegacyStatus("declined"); got != "withdrawn" {
		t.Fatalf("declined -> %s", got)
	}
	if got := normalizeLegacyStatus("applied"); got != "applied" {
		t.Fatalf("applied -> %s", got)
	}
}
