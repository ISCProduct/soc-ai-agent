package matching

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"Backend/internal/models"
)

// 「ずれた提案」を測る評価（SOCAIAGENT-670）。合成のペルソナと合成の企業だけを使う（実データは使わない）。
//
// ずれた提案 = 上位 N 件のうち、本人の傾向と企業の重視点が大きく離れた軸（fit.go の MismatchAxisCount）がある提案。
// 「ずれ1軸以上」と「参考（2軸以上）」の割合を、減点なし／ありで比べる。

var evalAxes = []string{"技術志向", "チームワーク志向", "リーダーシップ志向", "創造性志向", "安定志向", "成長志向", "ワークライフバランス", "チャレンジ志向", "細部志向", "コミュニケーション力"}

func syntheticCompanies(n int, seed int64) []*models.CompanyWeightProfile {
	r := rand.New(rand.NewSource(seed))
	w := func() int { return 35 + r.Intn(58) } // 実データの重視度の範囲（35〜92）
	out := make([]*models.CompanyWeightProfile, n)
	for i := range out {
		out[i] = &models.CompanyWeightProfile{
			TechnicalOrientation: w(), TeamworkOrientation: w(), LeadershipOrientation: w(), CreativityOrientation: w(),
			StabilityOrientation: w(), GrowthOrientation: w(), WorkLifeBalance: w(), ChallengeSeeking: w(),
			DetailOrientation: w(), CommunicationSkill: w(),
		}
	}
	return out
}

// 典型の人物像（合成）と、ランダムな人物像。評価済みの軸は 6〜10。
func syntheticPersonas(n int, seed int64) []map[string]float64 {
	archetypes := []map[string]float64{
		{"技術志向": 85, "創造性志向": 70, "細部志向": 75, "成長志向": 80, "コミュニケーション力": 35, "リーダーシップ志向": 30, "チームワーク志向": 45},
		{"コミュニケーション力": 85, "チームワーク志向": 80, "リーダーシップ志向": 70, "技術志向": 30, "細部志向": 40, "チャレンジ志向": 65},
		{"安定志向": 85, "ワークライフバランス": 80, "細部志向": 80, "チャレンジ志向": 30, "成長志向": 45, "創造性志向": 35},
		{"チャレンジ志向": 85, "成長志向": 85, "リーダーシップ志向": 75, "安定志向": 25, "ワークライフバランス": 35, "創造性志向": 70},
	}
	r := rand.New(rand.NewSource(seed))
	out := append([]map[string]float64{}, archetypes...)
	for len(out) < n {
		p := map[string]float64{}
		k := 6 + r.Intn(5)
		for _, i := range r.Perm(len(evalAxes))[:k] {
			p[evalAxes[i]] = float64(15 + r.Intn(75))
		}
		out = append(out, p)
	}
	return out
}

type evalResult struct{ off1, offRef, on1, onRef, topShift float64 }

func runFitEval(t *testing.T, topN int) evalResult {
	t.Helper()
	companies := syntheticCompanies(120, 7)
	personas := syntheticPersonas(60, 11)
	svc := &MatchingService{}

	topOf := func(persona map[string]float64) []*models.UserCompanyMatch {
		ms := make([]*models.UserCompanyMatch, len(companies))
		for i, c := range companies {
			ms[i] = svc.calculateMatchScore(persona, c, nil)
			ms[i].CompanyID = uint(i)
		}
		sort.SliceStable(ms, func(a, b int) bool { return ms[a].MatchScore > ms[b].MatchScore })
		return ms[:topN]
	}
	share := func(ms []*models.UserCompanyMatch, min int) float64 {
		n := 0
		for _, m := range ms {
			if MismatchAxisCount(axisMatchesOf(m)) >= min {
				n++
			}
		}
		return float64(n) / float64(len(ms))
	}

	var r evalResult
	for _, p := range personas {
		t.Setenv("MATCHING_FIT_PENALTY", "off")
		off := topOf(p)
		t.Setenv("MATCHING_FIT_PENALTY", "on")
		on := topOf(p)
		r.off1 += share(off, 1)
		r.offRef += share(off, ReferenceMismatchAxes)
		r.on1 += share(on, 1)
		r.onRef += share(on, ReferenceMismatchAxes)
		// 上位の入れ替わり（変えすぎていないか）: 上位 N 件のうち減点前と共通する企業の割合
		set := map[uint]bool{}
		for _, m := range off {
			set[m.CompanyID] = true
		}
		same := 0
		for _, m := range on {
			if set[m.CompanyID] {
				same++
			}
		}
		r.topShift += 1 - float64(same)/float64(topN)
	}
	n := float64(len(personas))
	r.off1, r.offRef, r.on1, r.onRef, r.topShift = r.off1/n, r.offRef/n, r.on1/n, r.onRef/n, r.topShift/n
	return r
}

func TestFitEval_MismatchedSuggestionsInTop10(t *testing.T) {
	r := runFitEval(t, 10)
	t.Log(fmt.Sprintf("上位10件に占める割合（60人×120社の合成）: ずれ1軸以上 %.1f%% → %.1f%%、参考（2軸以上）%.1f%% → %.1f%%、上位の入れ替わり %.1f%%",
		r.off1*100, r.on1*100, r.offRef*100, r.onRef*100, r.topShift*100))

	// 基準値（2026-11-18、合成）: ずれ1軸以上 16.5% → 1.7%、参考 1.2% → 0.0%、入れ替わり 14.8%。
	if r.on1 > r.off1/2 {
		t.Errorf("ずれた提案が十分に減っていない: %.1f%% → %.1f%%", r.off1*100, r.on1*100)
	}
	if r.onRef > 0.01 {
		t.Errorf("参考（2軸以上ずれ）の提案が上位10件に %.1f%% 残っている", r.onRef*100)
	}
	// 変えすぎない: 上位10件の入れ替わりは3割まで
	if r.topShift > 0.30 {
		t.Errorf("上位の入れ替わりが大きすぎる: %.1f%%", r.topShift*100)
	}
}

func TestMismatchAxisCountAndPenalty(t *testing.T) {
	tests := []struct {
		name    string
		axes    []float64
		want    int
		penalty float64
	}{
		{"ずれなし", []float64{90, 80, 70, 60, 50, 100, 55, 51, 80, 90}, 0, 0},
		{"境界（50ちょうどはずれない）", []float64{50, 50}, 0, 0},
		{"1軸", []float64{90, 49}, 1, 6},
		{"0は未計測なので数えない", []float64{0, 0, 30}, 1, 6},
		{"3軸で上限", []float64{10, 20, 30}, 3, 18},
		{"4軸でも上限のまま", []float64{10, 20, 30, 40}, 4, 18},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MismatchAxisCount(tt.axes); got != tt.want {
				t.Fatalf("MismatchAxisCount=%d, want %d", got, tt.want)
			}
			if got := FitPenalty(tt.want); got != tt.penalty {
				t.Fatalf("FitPenalty=%v, want %v", got, tt.penalty)
			}
		})
	}
}

func TestFitPenalty_CanBeSwitchedOff(t *testing.T) {
	user := map[string]float64{"技術志向": 90, "チームワーク志向": 90, "安定志向": 90, "成長志向": 90}
	company := &models.CompanyWeightProfile{TechnicalOrientation: 35, TeamworkOrientation: 35, StabilityOrientation: 90, GrowthOrientation: 90}
	svc := &MatchingService{}
	t.Setenv("MATCHING_FIT_PENALTY", "off")
	off := svc.calculateMatchScore(user, company, nil)
	t.Setenv("MATCHING_FIT_PENALTY", "")
	on := svc.calculateMatchScore(user, company, nil)
	if want := off.MatchScore - 12; on.MatchScore != want {
		t.Fatalf("2軸ずれ: off=%.1f on=%.1f（%.1f のはず）", off.MatchScore, on.MatchScore, want)
	}
}

func TestFitNote_ReferenceNeedsTwoAxes(t *testing.T) {
	one := &models.UserCompanyMatch{TechnicalMatch: 30, TeamworkMatch: 90}
	two := &models.UserCompanyMatch{TechnicalMatch: 30, StabilityMatch: 40, TeamworkMatch: 90}
	if FitNote(one) != "" {
		t.Fatalf("1軸のずれは参考にしない: %q", FitNote(one))
	}
	if got := FitNote(two); got != "参考: 技術志向・安定志向で、あなたの傾向と企業の重視点が大きく離れています。" {
		t.Fatalf("got %q", got)
	}
	rec := BuildRecommendations([]*models.UserCompanyMatch{{Company: models.Company{ID: 1}, TechnicalMatch: 30, StabilityMatch: 40}}, nil)
	if len(rec) != 1 || !rec[0].IsReference || rec[0].FitNote == "" {
		t.Fatalf("推薦に参考の印が付く: %+v", rec)
	}
}
