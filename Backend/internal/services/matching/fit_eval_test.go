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
// ずれた提案 = 上位 N 件のうち、本人の傾向と企業の重視点が大きく離れた軸（fit.go の MismatchAxes）がある提案。
// 企業の分布が 2 通りの場合で測る。
//   - ばらけた企業（各軸が 35〜92 に一様）
//   - 100 に張り付いた企業（実データの開発 DB に近い形。多くの軸が 100 付近で、4 軸だけがばらつく）

var evalAxes = axisNames[:]

func uniformCompanies(n int, seed int64) []*models.CompanyWeightProfile {
	r := rand.New(rand.NewSource(seed))
	w := func() int { return 35 + r.Intn(58) }
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

// 6 軸（技術・チームワーク・安定・成長・細部・コミュニケーション）が 100 付近に張り付き、
// 4 軸（リーダーシップ・創造性・ワークライフ・チャレンジ）だけが 10〜100 にばらつく。
func saturatedCompanies(n int, seed int64) []*models.CompanyWeightProfile {
	r := rand.New(rand.NewSource(seed))
	pin := func() int { return 94 + r.Intn(7) }
	vary := func() int { return 10 + r.Intn(91) }
	out := make([]*models.CompanyWeightProfile, n)
	for i := range out {
		out[i] = &models.CompanyWeightProfile{
			TechnicalOrientation: pin(), TeamworkOrientation: pin(), LeadershipOrientation: vary(), CreativityOrientation: vary(),
			StabilityOrientation: pin(), GrowthOrientation: pin(), WorkLifeBalance: vary(), ChallengeSeeking: vary(),
			DetailOrientation: pin(), CommunicationSkill: pin(),
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

type evalResult struct {
	off1, on1, offRef, onRef float64 // 上位N件に占める、ずれ1軸以上／参考（2軸以上）の割合
	topShift                 float64 // 減点前と減点後の、上位N件の入れ替わり
	allRef                   float64 // 全企業のうち「参考」になる割合
}

// runFitEval は、減点なし／あり（ばらつきを考慮）で上位 N 件を比べる。
// aware=false のときは、全軸を「ずれる軸」の対象にする（ばらつきを考慮しない版）。
func runFitEval(t *testing.T, companies []*models.CompanyWeightProfile, topN int, aware bool) evalResult {
	t.Helper()
	personas := syntheticPersonas(60, 11)
	svc := &MatchingService{}
	var inf *AxisFlags
	if aware {
		f := InformativeAxes(companies)
		inf = &f
	}

	rank := func(persona map[string]float64) []*models.UserCompanyMatch {
		ms := make([]*models.UserCompanyMatch, len(companies))
		for i, c := range companies {
			ms[i] = svc.calculateMatchScoreFor(persona, c, nil, inf)
			ms[i].CompanyID = uint(i)
		}
		sort.SliceStable(ms, func(a, b int) bool { return ms[a].MatchScore > ms[b].MatchScore })
		return ms
	}
	share := func(ms []*models.UserCompanyMatch, min int) float64 {
		n := 0
		for _, m := range ms {
			if len(mismatchAxisList(m)) >= min {
				n++
			}
		}
		return float64(n) / float64(len(ms))
	}

	var r evalResult
	for _, p := range personas {
		t.Setenv("MATCHING_FIT_PENALTY", "off")
		off := rank(p)
		t.Setenv("MATCHING_FIT_PENALTY", "on")
		on := rank(p)
		r.off1 += share(off[:topN], 1)
		r.offRef += share(off[:topN], ReferenceMismatchAxes)
		r.on1 += share(on[:topN], 1)
		r.onRef += share(on[:topN], ReferenceMismatchAxes)
		r.allRef += share(on, ReferenceMismatchAxes)
		set := map[uint]bool{}
		for _, m := range off[:topN] {
			set[m.CompanyID] = true
		}
		same := 0
		for _, m := range on[:topN] {
			if set[m.CompanyID] {
				same++
			}
		}
		r.topShift += 1 - float64(same)/float64(topN)
	}
	n := float64(len(personas))
	r.off1, r.on1, r.offRef, r.onRef, r.topShift, r.allRef = r.off1/n, r.on1/n, r.offRef/n, r.onRef/n, r.topShift/n, r.allRef/n
	return r
}

func logEval(t *testing.T, name string, r evalResult) {
	t.Log(fmt.Sprintf("%s 上位10件: ずれ1軸以上 %.1f%% → %.1f%%、参考 %.1f%% → %.1f%%、入れ替わり %.1f%%、全企業のうち参考 %.1f%%",
		name, r.off1*100, r.on1*100, r.offRef*100, r.onRef*100, r.topShift*100, r.allRef*100))
}

// ばらけた企業: 上位10件のずれた提案が減る。上位は変えすぎない。
func TestFitEval_SpreadCompanies(t *testing.T) {
	r := runFitEval(t, uniformCompanies(120, 7), 10, true)
	logEval(t, "ばらけた企業（60人×120社・合成）", r)
	// 基準値（2026-11-18）: ずれ1軸以上 16.5% → 1.7%、参考 1.2% → 0.0%、入れ替わり 14.8%
	if r.on1 > r.off1/2 {
		t.Errorf("ずれた提案が十分に減っていない: %.1f%% → %.1f%%", r.off1*100, r.on1*100)
	}
	if r.onRef > 0.01 {
		t.Errorf("参考の提案が上位10件に %.1f%% 残っている", r.onRef*100)
	}
	if r.topShift > 0.30 {
		t.Errorf("上位の入れ替わりが大きすぎる: %.1f%%", r.topShift*100)
	}
}

// 100 に張り付いた企業（実データに近い形）: ばらつきを考慮しないと、全社に「参考」が付く。
// 考慮すれば、ばらつきのある軸（4 軸）でのずれだけを数えるので、「参考」は一部の企業にとどまる。
func TestFitEval_SaturatedCompanies_DoesNotLabelEveryone(t *testing.T) {
	companies := saturatedCompanies(90, 5)
	naive := runFitEval(t, companies, 10, false)
	aware := runFitEval(t, companies, 10, true)
	logEval(t, "100に張り付いた企業・ばらつきを考慮しない", naive)
	logEval(t, "100に張り付いた企業・ばらつきを考慮する  ", aware)

	if naive.allRef < 0.5 {
		t.Fatalf("前提が崩れた: ばらつきを考慮しない版でも参考が少ない（%.1f%%）", naive.allRef*100)
	}
	// 考慮すれば、全企業のうち参考になる割合は半分未満
	if aware.allRef > 0.5 {
		t.Errorf("参考が多すぎる: 全企業の %.1f%%", aware.allRef*100)
	}
	// 4軸だけが判断に使える軸。ばらつきを考慮しない版より、参考が少ない
	if aware.allRef >= naive.allRef {
		t.Errorf("ばらつきを考慮しても参考が減っていない: %.1f%% → %.1f%%", naive.allRef*100, aware.allRef*100)
	}
	if aware.onRef > aware.offRef {
		t.Errorf("上位10件の参考が、減点で増えた: %.1f%% → %.1f%%", aware.offRef*100, aware.onRef*100)
	}
}

func TestInformativeAxes(t *testing.T) {
	// 全社が同じ値の軸は対象にしない。ばらつく軸だけが対象。
	cs := make([]*models.CompanyWeightProfile, 0, 10)
	for i := 0; i < 10; i++ {
		cs = append(cs, &models.CompanyWeightProfile{TechnicalOrientation: 100, TeamworkOrientation: 100 - i, CreativityOrientation: 10 + i*10, StabilityOrientation: 50})
	}
	got := InformativeAxes(cs)
	if got[0] || got[1] || got[4] {
		t.Fatalf("ほぼ定数の軸は対象外のはず: %v", got)
	}
	if !got[3] {
		t.Fatalf("ばらつく軸（創造性）は対象のはず: %v", got)
	}
	// 企業が少なすぎるときは、どの軸も対象にしない
	if few := InformativeAxes(cs[:MinCompaniesForSpread-1]); few != (AxisFlags{}) {
		t.Fatalf("企業が少ないときは対象なし: %v", few)
	}
}

func TestMismatchAxesAndPenalty(t *testing.T) {
	only3 := AxisFlags{3: true}
	tests := []struct {
		name string
		axes [10]float64
		inf  *AxisFlags
		want []string
		pen  float64
	}{
		{"ずれなし", [10]float64{90, 80, 70, 60, 50, 100, 55, 51, 80, 90}, nil, nil, 0},
		{"境界（50ちょうどはずれない）", [10]float64{50, 50}, nil, nil, 0},
		{"1軸", [10]float64{90, 49}, nil, []string{"チームワーク志向"}, 6},
		{"0は未計測なので数えない", [10]float64{0, 0, 30}, nil, []string{"リーダーシップ志向"}, 6},
		{"3軸で上限", [10]float64{10, 20, 30}, nil, []string{"技術志向", "チームワーク志向", "リーダーシップ志向"}, 18},
		{"ばらつきのある軸だけを数える", [10]float64{10, 20, 30, 40}, &only3, []string{"創造性志向"}, 6},
		{"ばらつきのある軸が無ければ数えない", [10]float64{10, 20, 30, 40}, &AxisFlags{}, nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MismatchAxes(tt.axes, tt.inf)
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Fatalf("MismatchAxes=%v, want %v", got, tt.want)
			}
			if p := FitPenalty(len(got)); p != tt.pen {
				t.Fatalf("FitPenalty=%v, want %v", p, tt.pen)
			}
		})
	}
	if FitPenalty(4) != MismatchPenaltyMax {
		t.Fatalf("4軸でも上限のまま")
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
	if on.FitMismatchAxes != "技術志向,チームワーク志向" || off.FitMismatchAxes != on.FitMismatchAxes {
		t.Fatalf("ずれる軸は減点の有無によらず保存する: off=%q on=%q", off.FitMismatchAxes, on.FitMismatchAxes)
	}
}

func TestFitNote_ReferenceNeedsTwoAxes(t *testing.T) {
	one := &models.UserCompanyMatch{FitMismatchAxes: "技術志向"}
	two := &models.UserCompanyMatch{FitMismatchAxes: "技術志向,安定志向"}
	if FitNote(one) != "" || FitNote(&models.UserCompanyMatch{}) != "" {
		t.Fatalf("1軸以下は参考にしない: %q", FitNote(one))
	}
	if got := FitNote(two); got != "参考: 技術志向・安定志向で、あなたの傾向と企業の重視点が大きく離れています。" {
		t.Fatalf("got %q", got)
	}
	rec := BuildRecommendations([]*models.UserCompanyMatch{{Company: models.Company{ID: 1}, FitMismatchAxes: "技術志向,安定志向"}}, nil)
	if len(rec) != 1 || !rec[0].IsReference || rec[0].FitNote == "" {
		t.Fatalf("推薦に参考の印が付く: %+v", rec)
	}
}
