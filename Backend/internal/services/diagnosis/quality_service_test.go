package diagnosis

import (
	"Backend/domain/entity"
	"testing"
)

func TestHeuristicFlags_FewAxesAndSaturated(t *testing.T) {
	scores := []entity.UserWeightScore{
		{WeightCategory: "技術志向", Score: 60},
		{WeightCategory: "成長志向", Score: 40},
	}
	matches := []*entity.UserCompanyMatch{
		{CompanyID: 1, MatchScore: 98, MatchedAxisCount: 2},
		{CompanyID: 2, MatchScore: 97, MatchedAxisCount: 2},
		{CompanyID: 3, MatchScore: 96, MatchedAxisCount: 2},
	}
	flags := heuristicFlags(scores, matches)
	want := map[string]bool{
		"few_measured_axes":   true,
		"saturated_matches":   true,
		"thin_match_evidence": true,
	}
	for _, f := range flags {
		if !want[f] {
			t.Fatalf("unexpected flag %q in %v", f, flags)
		}
		delete(want, f)
	}
	if len(want) > 0 {
		t.Fatalf("missing flags %v; got %v", want, flags)
	}
}

func TestHeuristicFlags_ZeroAxesIsThin(t *testing.T) {
	scores := []entity.UserWeightScore{
		{WeightCategory: "技術志向", Score: 60},
		{WeightCategory: "成長志向", Score: 40},
		{WeightCategory: "細部志向", Score: 50},
		{WeightCategory: "安定志向", Score: 55},
	}
	matches := []*entity.UserCompanyMatch{
		{CompanyID: 1, MatchScore: 80, MatchedAxisCount: 0},
		{CompanyID: 2, MatchScore: 70, MatchedAxisCount: 0},
	}
	flags := heuristicFlags(scores, matches)
	found := false
	for _, f := range flags {
		if f == "thin_match_evidence" {
			found = true
		}
	}
	if !found {
		t.Fatalf("MatchedAxisCount=0 should flag thin_match_evidence, got %v", flags)
	}
}

func TestHeuristicFlags_SingleMatch(t *testing.T) {
	scores := make([]entity.UserWeightScore, 6)
	for i := range scores {
		scores[i] = entity.UserWeightScore{WeightCategory: "c", Score: 50}
	}
	flags := heuristicFlags(scores, []*entity.UserCompanyMatch{
		{CompanyID: 1, MatchScore: 88, MatchedAxisCount: 6},
	})
	found := false
	for _, f := range flags {
		if f == "single_match_only" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected single_match_only, got %v", flags)
	}
}

func TestHeuristicFlags_Healthy(t *testing.T) {
	scores := make([]entity.UserWeightScore, 0, 8)
	for i := 0; i < 8; i++ {
		scores = append(scores, entity.UserWeightScore{WeightCategory: "c", Score: 50 + i})
	}
	matches := []*entity.UserCompanyMatch{
		{CompanyID: 1, MatchScore: 88, MatchedAxisCount: 8},
		{CompanyID: 2, MatchScore: 70, MatchedAxisCount: 8},
	}
	flags := heuristicFlags(scores, matches)
	if len(flags) != 0 {
		t.Fatalf("expected no flags, got %v", flags)
	}
}

func TestHeuristicFlags_ScoreZeroStillCounts(t *testing.T) {
	scores := []entity.UserWeightScore{
		{WeightCategory: "a", Score: 0},
		{WeightCategory: "b", Score: 0},
		{WeightCategory: "c", Score: 0},
		{WeightCategory: "d", Score: 0},
	}
	flags := heuristicFlags(scores, nil)
	for _, f := range flags {
		if f == "few_measured_axes" || f == "no_measured_axes" {
			t.Fatalf("score=0 rows are measured; unexpected %q in %v", f, flags)
		}
	}
}
