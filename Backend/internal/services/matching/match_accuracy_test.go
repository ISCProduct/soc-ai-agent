package matching

import (
	"math"
	"testing"

	"Backend/internal/models"
)

// 正確性の契約（#1124 / 診断妥当性）:
//
//  1. マッチ度は入力から手計算で再現できる
//  2. 未計測軸は「一致」にしない（スコア無しなら MatchScore=0）
//  3. 相性が良い企業は悪い企業より高い（順位が意味を持つ）
//  4. MatchedAxisCount は実際に平均に入れた軸数と一致する
//
// 「チャット回答が本人の真実を表すか」は検証不能。ここで保証するのは
// 計算の正確性と識別力だけ。

func TestMatchAccuracy_RecomputableFromInputs(t *testing.T) {
	svc := NewMatchingServiceForTest()

	user := map[string]float64{
		"技術志向":       60,
		"チームワーク志向":   60,
		"創造性志向":      40,
		"チャレンジ志向":    57,
		"コミュニケーション力": 80,
		"ワークライフバランス": 40,
	}
	company := &models.CompanyWeightProfile{
		TechnicalOrientation:  65,
		TeamworkOrientation:   60,
		LeadershipOrientation: 50,
		CreativityOrientation: 45,
		StabilityOrientation:  60,
		GrowthOrientation:     70,
		WorkLifeBalance:       50,
		ChallengeSeeking:      65,
		DetailOrientation:     55,
		CommunicationSkill:    70,
	}

	got := svc.calculateMatchScore(user, company)

	// 手計算: 計測6軸のみ
	wantParts := []float64{95, 100, 95, 90, 92, 90}
	var sum float64
	for _, p := range wantParts {
		sum += p
	}
	want := sum / float64(len(wantParts))

	if got.MatchedAxisCount != 6 {
		t.Fatalf("MatchedAxisCount=%d want 6", got.MatchedAxisCount)
	}
	if math.Abs(got.MatchScore-want) > 0.01 {
		t.Fatalf("MatchScore=%.4f want %.4f (hand-computed)", got.MatchScore, want)
	}
}

func TestMatchAccuracy_NoScoresMeansZeroNotNeutralAgreement(t *testing.T) {
	svc := NewMatchingServiceForTest()
	company := &models.CompanyWeightProfile{
		TechnicalOrientation: 65, TeamworkOrientation: 60, LeadershipOrientation: 50,
		CreativityOrientation: 45, StabilityOrientation: 60, GrowthOrientation: 70,
		WorkLifeBalance: 50, ChallengeSeeking: 65, DetailOrientation: 55, CommunicationSkill: 70,
	}
	got := svc.calculateMatchScore(map[string]float64{}, company)
	if got.MatchScore != 0 || got.MatchedAxisCount != 0 {
		t.Fatalf("empty scores must yield 0/0, got score=%.2f axes=%d",
			got.MatchScore, got.MatchedAxisCount)
	}
}

func TestMatchAccuracy_BetterFitRanksHigher(t *testing.T) {
	svc := NewMatchingServiceForTest()
	user := map[string]float64{
		"技術志向": 80, "チームワーク志向": 40, "成長志向": 70, "ワークライフバランス": 30,
	}
	goodFit := &models.CompanyWeightProfile{
		TechnicalOrientation: 80, TeamworkOrientation: 40, GrowthOrientation: 70, WorkLifeBalance: 30,
		LeadershipOrientation: 50, CreativityOrientation: 50, StabilityOrientation: 50,
		ChallengeSeeking: 50, DetailOrientation: 50, CommunicationSkill: 50,
	}
	badFit := &models.CompanyWeightProfile{
		TechnicalOrientation: 20, TeamworkOrientation: 90, GrowthOrientation: 20, WorkLifeBalance: 90,
		LeadershipOrientation: 50, CreativityOrientation: 50, StabilityOrientation: 50,
		ChallengeSeeking: 50, DetailOrientation: 50, CommunicationSkill: 50,
	}
	good := svc.calculateMatchScore(user, goodFit)
	bad := svc.calculateMatchScore(user, badFit)
	if good.MatchScore <= bad.MatchScore {
		t.Fatalf("good fit (%.1f) must beat bad fit (%.1f)", good.MatchScore, bad.MatchScore)
	}
	if good.MatchScore-bad.MatchScore < 20 {
		t.Fatalf("discrimination too weak: good=%.1f bad=%.1f delta=%.1f",
			good.MatchScore, bad.MatchScore, good.MatchScore-bad.MatchScore)
	}
}

func TestMatchAccuracy_PartialAxesDontInflateWithMissingNeutral(t *testing.T) {
	svc := NewMatchingServiceForTest()
	user := map[string]float64{"技術志向": 90}
	company := &models.CompanyWeightProfile{
		TechnicalOrientation: 90,
		TeamworkOrientation:  10, LeadershipOrientation: 10, CreativityOrientation: 10,
		StabilityOrientation: 10, GrowthOrientation: 10, WorkLifeBalance: 10,
		ChallengeSeeking: 10, DetailOrientation: 10, CommunicationSkill: 10,
	}
	got := svc.calculateMatchScore(user, company)
	if got.MatchedAxisCount != 1 {
		t.Fatalf("axes=%d want 1", got.MatchedAxisCount)
	}
	if math.Abs(got.MatchScore-100) > 0.01 {
		t.Fatalf("single perfect axis should be 100, got %.2f", got.MatchScore)
	}
}
