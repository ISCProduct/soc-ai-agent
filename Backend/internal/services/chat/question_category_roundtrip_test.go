package chat

import (
	"testing"

	"Backend/domain/valueobject"
)

// TestFallbackQuestions_ScoreBackToOwnCategory は、あるカテゴリのために出した
// 質問が、そのカテゴリのスコアとして採点されることを検証する（#929）。
//
// スコアの書き込み先は出題時の targetCategory ではなく、
// inferCategoryFromQuestion による質問文からの再推定で決まる。
// そのため質問文が他カテゴリのキーワードを含むと、
//
//	そのカテゴリの質問を出す → 回答が別カテゴリに書かれる →
//	当該カテゴリは未評価のまま → 次も同じカテゴリが選ばれる
//
// というループになり、診断が前に進まないままスコアが1カテゴリに堆積する。
// 質問文を書き換えるたびにこの整合が崩れうるので、テストで固定する。
func TestFallbackQuestions_ScoreBackToOwnCategory(t *testing.T) {
	s := &ChatService{}

	for _, c := range valueobject.AllWeightCategories() {
		category := string(c)
		for _, level := range []string{"新卒", "中途"} {
			questions := []string{s.fallbackQuestionForCategory(category, 0, level)}
			questions = append(questions, s.fallbackQuestionsForCategory(category, 0, level)...)

			for _, q := range questions {
				if q == "" {
					t.Errorf("%s(%s): 質問が空", category, level)
					continue
				}
				if got := s.inferCategoryFromQuestion(q); got != category {
					t.Errorf("%s(%s): 質問 %q が %q として採点される。\n"+
						"  この質問を出しても %s は未評価のままになり、出題がループする。\n"+
						"  質問文に %s のキーワードを含め、他カテゴリのキーワードを避けること。",
						category, level, q, got, category, category)
				}
			}
		}
	}
}

// inferCategoryFromQuestion が呼び出しごとに同じ結果を返すこと。
// map の反復順に依存していると、複数カテゴリのキーワードを含む質問で
// 採点先がリクエストごとに変わる。
func TestInferCategoryFromQuestion_IsDeterministic(t *testing.T) {
	s := &ChatService{}
	// 「新しい」(創造性志向) と「チーム」(チームワーク志向) の両方を含む。
	const ambiguous = "新しいことにチームで取り組んだ経験はありますか？"

	first := s.inferCategoryFromQuestion(ambiguous)
	for i := 0; i < 50; i++ {
		if got := s.inferCategoryFromQuestion(ambiguous); got != first {
			t.Fatalf("同じ質問で結果が変わる: %q と %q。map の反復順に依存している", first, got)
		}
	}
}
