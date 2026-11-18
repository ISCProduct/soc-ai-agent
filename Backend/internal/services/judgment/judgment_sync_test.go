package judgment_test

import (
	"testing"

	"Backend/internal/services/judgment"
	"Backend/internal/services/matching"
)

// 完了の下限は、推薦を暫定表示にする下限以上にする（下回ると、完了したのに暫定表示になる）。
func TestMinEvaluatedCategoriesNotBelowRecommendation(t *testing.T) {
	if judgment.MinEvaluatedCategories < matching.MinEvaluatedCategoriesForFinal {
		t.Fatalf("judgment=%d, matching=%d", judgment.MinEvaluatedCategories, matching.MinEvaluatedCategoriesForFinal)
	}
}
