package models

import (
	"testing"

	"Backend/domain/valueobject"
)

// TestWeightByCategory_CoversAllCanonicalCategories は、10軸のキーが
// 正典カテゴリと一致していることを検証する（#1027）。
//
// キーがタイポや表記揺れで引けないと weights[category] が 0 になり
// （中立50ではない）、その業界だけ全生徒で不当に低評価される。
// #929 が潰している「カテゴリ名の揺れで静かにスコアが壊れる」のと同じ穴。
func TestWeightByCategory_CoversAllCanonicalCategories(t *testing.T) {
	p := NeutralIndustryWeightProfile(1)
	weights := p.WeightByCategory()

	all := valueobject.AllWeightCategories()
	if len(weights) != len(all) {
		t.Errorf("キー数 = %d, want %d", len(weights), len(all))
	}
	for _, c := range all {
		v, ok := weights[string(c)]
		if !ok {
			t.Errorf("正典カテゴリ %q のキーが無い。この業界だけ 0 で評価される", c)
			continue
		}
		if v != 50 {
			t.Errorf("%q = %v, want 50（中立プロファイル）", c, v)
		}
	}
	// 正典に無いキーが紛れていないこと。
	canonical := map[string]bool{}
	for _, c := range all {
		canonical[string(c)] = true
	}
	for k := range weights {
		if !canonical[k] {
			t.Errorf("正典に無いキー %q が含まれる", k)
		}
	}
}

// 各軸の値が正しいカテゴリに割り当てられていること。
// 10個の代入を取り違えても件数チェックでは検出できない。
func TestWeightByCategory_MapsEachFieldCorrectly(t *testing.T) {
	p := &IndustryWeightProfile{
		TechnicalOrientation: 1, TeamworkOrientation: 2, LeadershipOrientation: 3,
		CreativityOrientation: 4, StabilityOrientation: 5, GrowthOrientation: 6,
		WorkLifeBalance: 7, ChallengeSeeking: 8, DetailOrientation: 9, CommunicationSkill: 10,
	}
	want := map[string]float64{
		"技術志向": 1, "チームワーク志向": 2, "リーダーシップ志向": 3, "創造性志向": 4,
		"安定志向": 5, "成長志向": 6, "ワークライフバランス": 7, "チャレンジ志向": 8,
		"細部志向": 9, "コミュニケーション力": 10,
	}
	got := p.WeightByCategory()
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%q = %v, want %v（軸の割り当てが入れ替わっている）", k, got[k], v)
		}
	}
}
