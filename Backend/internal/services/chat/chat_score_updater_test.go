package chat

import (
	"testing"

	"Backend/domain/valueobject"
)

// inferCategoryFromQuestionが返すカテゴリ名は、マッチングエンジン
// (domain/valueobject.WeightCategory)が参照するキーと一致していなければならない。
// #922: チームワーク/リーダーシップ/創造性の3カテゴリだけ「志向」サフィックスが
// 欠けており、マッチング計算でスコアが反映されない不具合があった。
func TestInferCategoryFromQuestion_MatchesCanonicalCategories(t *testing.T) {
	s := &ChatService{}
	cases := []struct {
		name     string
		question string
		want     valueobject.WeightCategory
	}{
		{"技術志向", "プログラミングを学んだきっかけを教えてください", valueobject.CategoryTechnical},
		{"チームワーク志向", "チームでの協力について教えてください", valueobject.CategoryTeamwork},
		{"リーダーシップ志向", "リーダーとして意思決定した経験は", valueobject.CategoryLeadership},
		{"創造性志向", "新しいアイデアを発想した経験は", valueobject.CategoryCreativity},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := s.inferCategoryFromQuestion(tt.question)
			if got != string(tt.want) {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}
