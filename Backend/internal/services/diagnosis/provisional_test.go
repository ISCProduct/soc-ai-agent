package diagnosis

import "testing"

func TestForcesProvisional(t *testing.T) {
	if !ForcesProvisional(40, nil) {
		t.Fatal("low confidence should force provisional")
	}
	if !ForcesProvisional(80, []string{"mostly_choice_only"}) {
		t.Fatal("weak chat flag should force provisional")
	}
	if ForcesProvisional(80, []string{"single_match_only"}) {
		t.Fatal("single_match_only alone should not force provisional")
	}
	if ForcesProvisional(0, nil) {
		t.Fatal("missing report (confidence 0) should not alone force")
	}
}

func TestParseFlagsJSON(t *testing.T) {
	got := ParseFlagsJSON(`["thin_chat_evidence","mostly_choice_only"]`)
	if len(got) != 2 || got[0] != "thin_chat_evidence" {
		t.Fatalf("got %v", got)
	}
	if ParseFlagsJSON("") != nil {
		t.Fatal("empty should be nil")
	}
}
