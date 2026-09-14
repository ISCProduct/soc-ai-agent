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
	want := map[string]bool{"few_measured_axes": true, "saturated_matches": true, "thin_match_evidence": true}
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
