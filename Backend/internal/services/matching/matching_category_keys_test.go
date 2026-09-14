package matching

import (
	"testing"

	"Backend/domain/valueobject"
	"Backend/internal/models"
)

// TestCalculateMatchScore_UsesCanonicalKeys は、マッチング計算が引くキーが
// 正典10種と一致することを検証する（#929）。
//
// ここが正典とずれると scoredMatch の `!ok` 分岐で未計測として除外され、
// エラーもログも出ないままユーザーの実スコアが捨てられる（#1124 以前は中立50で埋めていた）。
// 「静かに壊れる」ため、テストで固定しておく必要がある。
func TestCalculateMatchScore_UsesCanonicalKeys(t *testing.T) {
	// 正典キーだけを持つスコアマップ。全カテゴリ100、企業側も100にすると
	// 全カテゴリがヒットしたときに算出軸数が10になる。
	userScores := map[string]float64{}
	for _, c := range valueobject.AllWeightCategories() {
		userScores[string(c)] = 100
	}
	profile := &models.CompanyWeightProfile{
		TechnicalOrientation:  100,
		TeamworkOrientation:   100,
		LeadershipOrientation: 100,
		CreativityOrientation: 100,
		StabilityOrientation:  100,
		GrowthOrientation:     100,
		WorkLifeBalance:       100,
		ChallengeSeeking:      100,
		DetailOrientation:     100,
		CommunicationSkill:    100,
	}

	svc := &MatchingService{}
	full := svc.calculateMatchScore(userScores, profile)

	// 1カテゴリずつ抜いて、必ず算出軸数が減ることを確認する。
	// 減らないキーがあれば、そのカテゴリは calculateMatchScore から
	// 引かれていない（＝正典とキーがずれている）。
	for _, c := range valueobject.AllWeightCategories() {
		partial := map[string]float64{}
		for k, v := range userScores {
			if k != string(c) {
				partial[k] = v
			}
		}
		got := svc.calculateMatchScore(partial, profile)
		// 未計測カテゴリは平均から除外されるため MatchScore は変わらない（#1124）。
		// キーが引けているかは「算出に使えた軸の数」が減るかで判定する。
		if got.MatchedAxisCount >= full.MatchedAxisCount {
			t.Errorf("カテゴリ %q を外しても算出軸数が減らない (%d >= %d)。"+
				"calculateMatchScore がこのキーを引いていない可能性がある",
				c, got.MatchedAxisCount, full.MatchedAxisCount)
		}
	}
}

// 未計測カテゴリが平均から除外されることを固定する。
// この挙動があるため、キーがずれても「エラーにならず静かに希釈される」。
func TestScoredMatch_MissingCategoryIsExcluded(t *testing.T) {
	// 未計測カテゴリは平均に含めない（#1124）。
	// 以前は中立値(50)で埋めて評価件数に数えていたため、スコアを1つも持たない
	// ユーザーでも企業の重視度(45〜92)と近くなり、全社97%前後に固まっていた。
	score, count, total := scoredMatch(map[string]float64{}, "技術志向", 50, 0, 0)
	if score != 0 {
		t.Errorf("未計測カテゴリのスコア = %.2f, want 0", score)
	}
	if count != 0 {
		t.Errorf("未計測カテゴリが評価件数に数えられている: got %d, want 0", count)
	}
	if total != 0 {
		t.Errorf("未計測カテゴリが合計に加算されている: got %.2f, want 0", total)
	}

	// 計測済みなら従来どおり数える
	score2, count2, total2 := scoredMatch(map[string]float64{"技術志向": 50}, "技術志向", 50, 0, 0)
	if score2 != 100 || count2 != 1 || total2 != 100 {
		t.Errorf("計測済み = (%.2f, %d, %.2f), want (100, 1, 100)", score2, count2, total2)
	}
}
