package interview

import (
	"testing"

	"Backend/domain/valueobject"
)

func TestEvaluateSpeechHeuristic_UsesOfficialCategories(t *testing.T) {
	t.Parallel()
	res := EvaluateSpeechHeuristic("具体的なプログラミング経験があり、チームで協力して説明もわかりやすく伝えました")
	if len(res) == 0 {
		t.Fatal("expected some category deltas")
	}
	official := map[string]struct{}{}
	for _, c := range valueobject.AllWeightCategories() {
		official[string(c)] = struct{}{}
	}
	for cat := range res {
		if _, ok := official[cat]; !ok {
			t.Fatalf("unexpected category %q (must match user_weight_scores / CompanyWeightProfile names)", cat)
		}
	}
	if res[string(valueobject.CategoryTechnical)] == 0 {
		t.Fatal("expected 技術志向 delta from プログラミング")
	}
	if res[string(valueobject.CategoryCommunication)] == 0 {
		t.Fatal("expected コミュニケーション力 delta")
	}
}

func TestEvaluateSpeechHeuristic_AllTenCategoriesCovered(t *testing.T) {
	t.Parallel()
	if got := len(valueobject.AllWeightCategories()); got != 10 {
		t.Fatalf("AllWeightCategories len=%d want 10", got)
	}
}
