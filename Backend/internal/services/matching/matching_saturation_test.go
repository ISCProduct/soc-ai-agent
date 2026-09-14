package matching

import (
	"math"
	"testing"

	"Backend/internal/models"
)

// TestCalculateCategoryMatch_IsLinear は差がそのまま減点になることを固定する（#1124）。
//
// 以前はロジスティック関数（k=12）で、実際に現れる差の範囲がすべて平坦部に入り
// 差20で97.3、差30で91.7とほとんど差が出なかった。
func TestCalculateCategoryMatch_IsLinear(t *testing.T) {
	tests := []struct {
		user, company, want float64
	}{
		{user: 50, company: 50, want: 100},
		{user: 60, company: 50, want: 90},
		{user: 70, company: 50, want: 80},
		{user: 80, company: 50, want: 70},
		{user: 100, company: 50, want: 50},
		{user: 0, company: 100, want: 0},
		{user: 100, company: 0, want: 0},
		// 対称であること（どちらが高くても同じ差なら同じ）
		{user: 40, company: 50, want: 90},
	}

	for _, tt := range tests {
		got := CalculateCategoryMatch(tt.user, tt.company)
		if math.Abs(got-tt.want) > 0.001 {
			t.Errorf("CalculateCategoryMatch(%.0f, %.0f) = %.2f, want %.2f",
				tt.user, tt.company, got, tt.want)
		}
	}
}

// TestCalculateCategoryMatch_Discriminates は差が広がれば点数も実質的に下がることを固定する。
//
// 飽和の再発検知が目的。差20と差40で10ポイント以上開かなければ、
// また「どの企業とも高相性」に戻っている。
func TestCalculateCategoryMatch_Discriminates(t *testing.T) {
	near := CalculateCategoryMatch(50, 70) // 差20
	far := CalculateCategoryMatch(50, 90)  // 差40
	if spread := near - far; spread < 10 {
		t.Errorf("差20と差40の開き = %.1f ポイントしかない（飽和している）: near=%.1f far=%.1f",
			spread, near, far)
	}
}

// TestCalculateMatchScore_ExcludesUnmeasured は未計測軸を平均に含めないことを検証する（#1124）。
//
// 以前は未計測を中立値(50)で埋めて10軸の平均を取っていた。企業の重視度が
// 45〜92 に寄っている実データでは中立値との差が小さく、スコアを1つも持たない
// ユーザーでも全企業と97%前後で一致していた。
func TestCalculateMatchScore_ExcludesUnmeasured(t *testing.T) {
	profile := &models.CompanyWeightProfile{
		TechnicalOrientation:  60,
		TeamworkOrientation:   60,
		LeadershipOrientation: 60,
		CreativityOrientation: 60,
		StabilityOrientation:  60,
		GrowthOrientation:     60,
		WorkLifeBalance:       60,
		ChallengeSeeking:      60,
		DetailOrientation:     60,
		CommunicationSkill:    60,
	}
	svc := &MatchingService{}

	t.Run("スコアが1つも無ければ算出できない", func(t *testing.T) {
		got := svc.calculateMatchScore(map[string]float64{}, profile)
		if got.EvaluatedCategories != 0 {
			t.Errorf("EvaluatedCategories = %d, want 0", got.EvaluatedCategories)
		}
		// 以前はここが 97 前後になっていた
		if got.MatchScore != 0 {
			t.Errorf("MatchScore = %.1f, want 0（計測軸ゼロで高スコアを出さない）", got.MatchScore)
		}
	})

	t.Run("計測済みの軸だけで平均する", func(t *testing.T) {
		// 技術志向だけ計測済み。差20 -> 80点
		got := svc.calculateMatchScore(map[string]float64{"技術志向": 80}, profile)
		if got.EvaluatedCategories != 1 {
			t.Errorf("EvaluatedCategories = %d, want 1", got.EvaluatedCategories)
		}
		if math.Abs(got.MatchScore-80) > 0.001 {
			t.Errorf("MatchScore = %.2f, want 80（未計測9軸に引っ張られない）", got.MatchScore)
		}
		if math.Abs(got.TechnicalMatch-80) > 0.001 {
			t.Errorf("TechnicalMatch = %.2f, want 80", got.TechnicalMatch)
		}
		if got.TeamworkMatch != 0 {
			t.Errorf("未計測軸のスコア = %.2f, want 0", got.TeamworkMatch)
		}
	})

	t.Run("2軸計測ならその平均", func(t *testing.T) {
		// 技術志向: 差20 -> 80 / コミュニケーション力: 差0 -> 100
		got := svc.calculateMatchScore(
			map[string]float64{"技術志向": 80, "コミュニケーション力": 60}, profile)
		if got.EvaluatedCategories != 2 {
			t.Errorf("EvaluatedCategories = %d, want 2", got.EvaluatedCategories)
		}
		if math.Abs(got.MatchScore-90) > 0.001 {
			t.Errorf("MatchScore = %.2f, want 90", got.MatchScore)
		}
	})
}

// TestCalculateMatchScore_SpreadsAcrossCompanies は企業ごとに差が出ることを固定する。
//
// 同じユーザーが「合う企業」と「合わない企業」で大きく違うスコアになること。
// ここが縮むと推薦の並びが意味を失う。
func TestCalculateMatchScore_SpreadsAcrossCompanies(t *testing.T) {
	user := map[string]float64{"技術志向": 90, "ワークライフバランス": 20}

	techDriven := &models.CompanyWeightProfile{TechnicalOrientation: 90, WorkLifeBalance: 20}
	balanceDriven := &models.CompanyWeightProfile{TechnicalOrientation: 30, WorkLifeBalance: 85}

	svc := &MatchingService{}
	good := svc.calculateMatchScore(user, techDriven).MatchScore
	bad := svc.calculateMatchScore(user, balanceDriven).MatchScore

	if spread := good - bad; spread < 30 {
		t.Errorf("合う企業と合わない企業の差 = %.1f ポイントしかない（飽和している）: good=%.1f bad=%.1f",
			spread, good, bad)
	}
}
