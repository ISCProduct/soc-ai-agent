package assessment

import (
	"maps"
	"testing"

	"Backend/internal/models"
)

func q(id uint, cat string, reverse bool) models.AssessmentQuestion {
	return models.AssessmentQuestion{ID: id, WeightCategory: cat, ReverseScored: reverse}
}

func TestScoreAnswers(t *testing.T) {
	qs := []models.AssessmentQuestion{
		q(1, "技術志向", false),
		q(2, "技術志向", true),
		q(3, "安定志向", false),
		q(4, "成長志向", false),
	}
	tests := []struct {
		name    string
		answers map[uint]int
		want    map[string]float64
	}{
		// 5→100, 逆転1→(6-1=5)→100
		{"逆転項目は反転して換算する", map[uint]int{1: 5, 2: 1, 3: 3, 4: 1}, map[string]float64{"技術志向": 100, "安定志向": 50, "成長志向": 0}},
		// 4→75, 逆転4→2→25 → 平均50
		{"同じカテゴリは平均", map[uint]int{1: 4, 2: 4, 3: 3, 4: 1}, map[string]float64{"技術志向": 50, "安定志向": 50, "成長志向": 0}},
		// 未回答のカテゴリは0で埋めない
		{"答えていないカテゴリは結果に出さない", map[uint]int{3: 5}, map[string]float64{"安定志向": 100}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ScoreAnswers(qs, tt.answers)
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if !maps.Equal(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScoreAnswers_Rejects(t *testing.T) {
	tests := []struct {
		name    string
		qs      []models.AssessmentQuestion
		answers map[uint]int
	}{
		{"正典外のカテゴリ", []models.AssessmentQuestion{q(1, "やる気", false)}, map[uint]int{1: 3}},
		{"回答値が範囲外（0）", []models.AssessmentQuestion{q(1, "技術志向", false)}, map[uint]int{1: 0}},
		{"回答値が範囲外（6）", []models.AssessmentQuestion{q(1, "技術志向", false)}, map[uint]int{1: 6}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ScoreAnswers(tt.qs, tt.answers); err == nil {
				t.Fatal("エラーにならなかった")
			}
		})
	}
}

// 別名は正典へ寄せて保存する（ParseWeightCategory 経由）。
func TestScoreAnswers_NormalizesAlias(t *testing.T) {
	got, err := ScoreAnswers([]models.AssessmentQuestion{q(1, "チームワーク", false)}, map[uint]int{1: 5})
	if err != nil || got["チームワーク志向"] != 100 || len(got) != 1 {
		t.Fatalf("got %v err %v", got, err)
	}
}

func TestMergeScores(t *testing.T) {
	tests := []struct {
		name       string
		chat, test map[string]float64
		want       map[string]float64
	}{
		{"両方あれば0.5:0.5", map[string]float64{"技術志向": 40}, map[string]float64{"技術志向": 80}, map[string]float64{"技術志向": 60}},
		{"チャットだけのカテゴリはそのまま", map[string]float64{"安定志向": 70}, map[string]float64{}, map[string]float64{"安定志向": 70}},
		{"検査だけのカテゴリはそのまま", map[string]float64{}, map[string]float64{"成長志向": 30}, map[string]float64{"成長志向": 30}},
		{"どちらにも無いカテゴリは出さない（0埋めしない）", map[string]float64{"技術志向": 50}, map[string]float64{"成長志向": 50}, map[string]float64{"技術志向": 50, "成長志向": 50}},
		{"両方空なら空", nil, nil, map[string]float64{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeScores(tt.chat, tt.test, ChatWeight)
			if !maps.Equal(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// 合流は入力を書き換えない（チャット由来のスコアを壊さない）。
func TestMergeScores_DoesNotMutateInputs(t *testing.T) {
	chat := map[string]float64{"技術志向": 40}
	MergeScores(chat, map[string]float64{"技術志向": 80}, ChatWeight)
	if chat["技術志向"] != 40 {
		t.Fatalf("chat mutated: %v", chat)
	}
}
