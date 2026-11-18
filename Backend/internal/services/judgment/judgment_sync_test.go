package judgment_test

import (
	"testing"

	"Backend/internal/services/judgment"
	"Backend/internal/services/matching"
)

// 下限は、推薦を暫定表示にする下限と同じ値を使う（食い違うと、完了なのに暫定表示になる）。
func TestMinEvaluatedCategoriesMatchesRecommendation(t *testing.T) {
	if judgment.MinEvaluatedCategories != matching.MinEvaluatedCategoriesForFinal {
		t.Fatalf("judgment=%d, matching=%d", judgment.MinEvaluatedCategories, matching.MinEvaluatedCategoriesForFinal)
	}
}
