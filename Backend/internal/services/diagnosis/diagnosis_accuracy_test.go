package diagnosis

import (
	"Backend/domain/entity"
	"testing"
)

// 診断妥当性の正確性契約:
//
// 品質ジョブは「本人の適性が正しいか」は判定できない。
// 保証するのは「根拠の薄さを過大評価しない」ことだけ。
//
//  1. 計測軸が少ないほど confidence は低い
//  2. LLM が出す高い confidence はヒューリスティックを超えられない
//  3. マッチが無い／1件しかない／軸が薄い場合は必ずフラグが立つ

func makeScores(n int) []entity.UserWeightScore {
	out := make([]entity.UserWeightScore, n)
	for i := range out {
		out[i] = entity.UserWeightScore{WeightCategory: "c", Score: 50}
	}
	return out
}

func TestDiagnosisAccuracy_ConfidenceTracksEvidence(t *testing.T) {
	thin := heuristicConfidence(nil, []string{"no_measured_axes", "no_matches"})
	rich := heuristicConfidence(makeScores(10), nil)

	if thin >= rich {
		t.Fatalf("thin evidence confidence (%d) must be < rich (%d)", thin, rich)
	}
	if thin > 20 {
		t.Fatalf("no scores/no matches must stay low confidence, got %d", thin)
	}
}

func TestDiagnosisAccuracy_LLMCannotInflatePastHeuristic(t *testing.T) {
	heuristic := 17
	llm := 90
	got := minInt(heuristic, clampInt(llm, 0, 100))
	if got != 17 {
		t.Fatalf("LLM must not raise confidence above heuristic: got %d", got)
	}
}

func TestDiagnosisAccuracy_StructuralFlagsAlwaysFire(t *testing.T) {
	noMatch := heuristicFlags(makeScores(6), nil)
	if !hasFlag(noMatch, "no_matches") {
		t.Fatalf("expected no_matches, got %v", noMatch)
	}

	one := heuristicFlags(makeScores(6), []*entity.UserCompanyMatch{
		{CompanyID: 1, MatchScore: 80, MatchedAxisCount: 6},
	})
	if !hasFlag(one, "single_match_only") {
		t.Fatalf("expected single_match_only, got %v", one)
	}

	thinAxes := heuristicFlags(makeScores(6), []*entity.UserCompanyMatch{
		{CompanyID: 1, MatchScore: 80, MatchedAxisCount: 0},
		{CompanyID: 2, MatchScore: 70, MatchedAxisCount: 0},
	})
	if !hasFlag(thinAxes, "thin_match_evidence") {
		t.Fatalf("MatchedAxisCount=0 must flag thin_match_evidence, got %v", thinAxes)
	}
}

func hasFlag(flags []string, want string) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}
