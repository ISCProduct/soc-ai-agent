package matching

import (
	"Backend/internal/models"
	"testing"
)

type stubCalibrationReader struct {
	weights []models.ScoreCalibrationWeight
	err     error
}

func (s stubCalibrationReader) GetLatestCalibrationWeights() ([]models.ScoreCalibrationWeight, error) {
	return s.weights, s.err
}

func TestCalculateMatchScore_AppliesCalibrationAxisWeights(t *testing.T) {
	svc := &MatchingService{}
	user := map[string]float64{
		"技術志向": 80, "チームワーク志向": 50, "リーダーシップ志向": 50,
		"創造性志向": 50, "安定志向": 50, "成長志向": 50,
		"ワークライフバランス": 50, "チャレンジ志向": 50, "細部志向": 50, "コミュニケーション力": 50,
	}
	profile := &models.CompanyWeightProfile{
		TechnicalOrientation: 90, TeamworkOrientation: 50, LeadershipOrientation: 50,
		CreativityOrientation: 50, StabilityOrientation: 50, GrowthOrientation: 50,
		WorkLifeBalance: 50, ChallengeSeeking: 50, DetailOrientation: 50, CommunicationSkill: 50,
	}

	base := svc.calculateMatchScore(user, profile, nil).MatchScore
	// 技術軸だけ一致が低い（差10→90、他軸は100）。重みを上げると総合は下がる。
	boosted := svc.calculateMatchScore(user, profile, map[string]float64{"技術志向": 3.0}).MatchScore
	if boosted >= base {
		t.Fatalf("期待: boosted < base（低一致軸の重み増）, got base=%.2f boosted=%.2f", base, boosted)
	}
}

func TestLoadCalibrationAxisWeights_ClampsAndSkipsInactive(t *testing.T) {
	svc := &MatchingService{}
	svc.SetCalibrationWeightReader(stubCalibrationReader{weights: []models.ScoreCalibrationWeight{
		{Category: "技術志向", Weight: 99, IsActive: true},
		{Category: "安定志向", Weight: 0.01, IsActive: true},
		{Category: "成長志向", Weight: 2.0, IsActive: false},
		{Category: "", Weight: 2.0, IsActive: true},
	}})
	got := svc.loadCalibrationAxisWeights()
	if got["技術志向"] != 3.0 {
		t.Errorf("技術志向 clamp want 3.0 got %v", got["技術志向"])
	}
	if got["安定志向"] != 0.1 {
		t.Errorf("安定志向 clamp want 0.1 got %v", got["安定志向"])
	}
	if _, ok := got["成長志向"]; ok {
		t.Errorf("inactive 成長志向 must be omitted")
	}
	if _, ok := got[""]; ok {
		t.Errorf("empty category must be omitted")
	}
}
