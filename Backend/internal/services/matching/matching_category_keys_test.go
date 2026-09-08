package matching

import (
	"testing"

	"Backend/domain/valueobject"
	"Backend/internal/models"
)

// TestCalculateMatchScore_UsesCanonicalKeys は、マッチング計算が引くキーが
// 正典10種と一致することを検証する（#929）。
//
// ここが正典とずれると scoredMatch の `!ok` 分岐で中立50に置き換わり、
// エラーもログも出ないままユーザーの実スコアが捨てられる。
// 「静かに壊れる」ため、テストで固定しておく必要がある。
func TestCalculateMatchScore_UsesCanonicalKeys(t *testing.T) {
	// 正典キーだけを持つスコアマップ。全カテゴリ100、企業側も100にすると
	// 全カテゴリがヒットした場合のみ総合スコアが最大になる。
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

	// 1カテゴリずつ抜いて、必ずスコアが下がることを確認する。
	// 下がらないキーがあれば、そのカテゴリは calculateMatchScore から
	// 引かれていない（＝正典とキーがずれている）。
	for _, c := range valueobject.AllWeightCategories() {
		partial := map[string]float64{}
		for k, v := range userScores {
			if k != string(c) {
				partial[k] = v
			}
		}
		got := svc.calculateMatchScore(partial, profile)
		if got.MatchScore >= full.MatchScore {
			t.Errorf("カテゴリ %q を外してもスコアが下がらない (%.2f >= %.2f)。"+
				"calculateMatchScore がこのキーを引いていない可能性がある",
				c, got.MatchScore, full.MatchScore)
		}
	}
}

// 未評価カテゴリが中立50として扱われることを固定する。
// この挙動があるため、キーがずれても「エラーにならず静かに希釈される」。
func TestScoredMatch_MissingCategoryUsesNeutral(t *testing.T) {
	score, count, total := scoredMatch(map[string]float64{}, "技術志向", 50, 0, 0)
	neutral, _, _ := scoredMatch(map[string]float64{"技術志向": 50}, "技術志向", 50, 0, 0)
	if score != neutral {
		t.Errorf("未評価カテゴリが中立50として扱われていない: got %.2f want %.2f", score, neutral)
	}
	if count != 1 {
		t.Errorf("未評価でも評価件数に数えられるはず: got %d", count)
	}
	if total != score {
		t.Errorf("合計に加算されるはず: got %.2f want %.2f", total, score)
	}
}
