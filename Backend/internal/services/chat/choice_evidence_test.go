package chat

import "testing"

func TestAdjustChoiceAxisScore_ChoiceOnlyDampensExtremes(t *testing.T) {
	got, flags := AdjustChoiceAxisScore(100, "")
	// 50 + 50*0.55 = 77.5 → 78
	if got != 78 {
		t.Fatalf("A without reason: got %d want 78", got)
	}
	if !containsFlag(flags, "choice_only_evidence") {
		t.Fatalf("flags=%v", flags)
	}

	gotE, _ := AdjustChoiceAxisScore(20, "")
	// 50 + (20-50)*0.55 = 33.5 → 34
	if gotE != 34 {
		t.Fatalf("E without reason: got %d want 34", gotE)
	}
}

func TestAdjustChoiceAxisScore_SupportingReasonKeepsFull(t *testing.T) {
	got, flags := AdjustChoiceAxisScore(100, "チームで進めるのが好きで、調整役として動いてきました")
	if got != 100 {
		t.Fatalf("supporting reason should keep 100, got %d", got)
	}
	if !containsFlag(flags, "choice_with_supporting_reason") {
		t.Fatalf("flags=%v", flags)
	}
}

func TestAdjustChoiceAxisScore_ContradictionDampensHard(t *testing.T) {
	got, flags := AdjustChoiceAxisScore(100, "本当はチームワークが苦手で一人で進めたいです")
	// 50 + 50*0.25 = 62.5 → 62 or 63
	if got < 60 || got > 65 {
		t.Fatalf("contradiction should dampen near 62, got %d", got)
	}
	if !containsFlag(flags, "choice_reason_contradiction") {
		t.Fatalf("flags=%v", flags)
	}
}

func containsFlag(flags []string, want string) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}
