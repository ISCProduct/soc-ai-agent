package flywheel

import (
	"testing"

	"Backend/domain/valueobject"
)

// TestInterviewScoreMapping_TargetsAreCanonical は、面接レポートから
// user_weight_scores へ反映するときの写像先が正典であることを検証する（#929）。
//
// ここが正典外だと、面接で得たスコアが weight_category に書かれても
// マッチング側から一度も引かれない死んだ行になる。
// 実際に「リーダーシップ」「チームワーク」（志向なし）が書き込まれ、
// DB に不正な行が生成され続けていた。
func TestInterviewScoreMapping_TargetsAreCanonical(t *testing.T) {
	canonical := map[string]bool{}
	for _, c := range valueobject.AllWeightCategories() {
		canonical[string(c)] = true
	}

	if len(interviewScoreMapping) == 0 {
		t.Fatal("写像が空。テストが対象を見失っている")
	}
	for _, m := range interviewScoreMapping {
		if len(m.categories) == 0 {
			t.Errorf("%s: 反映先カテゴリが空", m.interviewKey)
		}
		for _, c := range m.categories {
			if !canonical[c] {
				t.Errorf("%s: 正典外のカテゴリ %q に反映しようとしている。"+
					"domain/valueobject/match.go の10種に揃えること", m.interviewKey, c)
			}
		}
	}
}
