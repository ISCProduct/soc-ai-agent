package chat

import (
	"testing"

	"Backend/domain/valueobject"
	"Backend/internal/models"
)

func TestPickPhaseTarget_未採点の軸を繰り返さず次の軸へ進む(t *testing.T) {
	allowed := phaseCategories["aptitude_analysis"]
	scores := map[string]int{}
	asked := map[string]int{}

	first, _ := pickPhaseTarget(allowed, scores, asked)
	if first != "コミュニケーション力" {
		t.Fatalf("first = %s", first)
	}
	asked[first] = 1

	second, _ := pickPhaseTarget(allowed, scores, asked)
	if second != "チームワーク志向" {
		t.Fatalf("second = %s, want チームワーク志向", second)
	}
}

func TestPickPhaseTarget_前のフェーズで聞いた軸より未出題を優先する(t *testing.T) {
	asked := map[string]int{"技術志向": 1, "創造性志向": 1, "成長志向": 1}
	got, _ := pickPhaseTarget(phaseCategories["interest_analysis"], map[string]int{}, asked)
	if got != "チャレンジ志向" {
		t.Fatalf("got %s, want チャレンジ志向", got)
	}
}

func TestPhaseBudget_スコアが付かなくても全軸を1回は聞く(t *testing.T) {
	budgets := []struct {
		phase string
		n     int
	}{
		{"job_analysis", 4},
		{"interest_analysis", 4},
		{"aptitude_analysis", 4},
		{"future_analysis", 3},
	}
	asked := map[string]int{}
	scores := map[string]int{}
	for _, b := range budgets {
		for range b.n {
			cat, _ := pickPhaseTarget(phaseCategories[b.phase], scores, asked)
			if cat == "" {
				t.Fatalf("%s picked an empty category", b.phase)
			}
			asked[cat]++
		}
	}
	for _, c := range valueobject.AllWeightCategories() {
		if asked[string(c)] < 1 {
			t.Errorf("%s was never asked: %v", c, asked)
		}
	}
}

func TestAskedCategoryCounts_案内メッセージは数えない(t *testing.T) {
	history := []models.ChatMessage{
		{Role: "assistant", Content: "内容を書いてください。", WeightCategory: ""},
		{Role: "user", Content: "A", WeightCategory: "技術志向"},
		{Role: "assistant", Content: "技術の質問", WeightCategory: "技術志向"},
	}
	counts := askedCategoryCounts(history)
	if counts["技術志向"] != 1 {
		t.Fatalf("counts = %v", counts)
	}
}
