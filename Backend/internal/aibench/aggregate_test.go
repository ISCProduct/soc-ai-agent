package aibench

import (
	"math"
	"slices"
	"testing"
)

// 集計は LLM を呼ばずに固定する。指標の計算が壊れても実行結果は返ってくるため、
// ここが無いと「数字は出るが意味が違う」状態に気付けない。
func TestAggregate(t *testing.T) {
	obs := []Observation{
		// bad を2回: スコア 0.2 / 0.3（σ=0.05）
		{CaseID: "c-bad", Label: LabelBad, Run: 1, Score: 0.2, LatencyMS: 100, PromptTokens: 1000, CompletionTokens: 500},
		{CaseID: "c-bad", Label: LabelBad, Run: 2, Score: 0.3, LatencyMS: 200, PromptTokens: 1000, CompletionTokens: 500},
		// mid を2回: 1回は破損
		{CaseID: "c-mid", Label: LabelMid, Run: 1, Score: 0.5, LatencyMS: 300, PromptTokens: 1000, CompletionTokens: 500,
			Violations: []string{"json_not_bare"}},
		{CaseID: "c-mid", Label: LabelMid, Run: 2, LatencyMS: 400, Broken: true, BrokenReason: BrokenTruncated},
		// good を2回: スコア 0.8 固定（σ=0）
		{CaseID: "c-good", Label: LabelGood, Run: 1, Score: 0.8, LatencyMS: 500, PromptTokens: 1000, CompletionTokens: 500},
		{CaseID: "c-good", Label: LabelGood, Run: 2, Score: 0.8, LatencyMS: 600, PromptTokens: 1000, CompletionTokens: 500},
	}
	s := Aggregate(TargetResume, "gpt-4o-mini", "openai:/responses", "2026-09-28T00:00:00Z", obs)

	if s.Cases != 3 || s.Runs != 2 {
		t.Errorf("件数/回数 = %d/%d, want 3/2", s.Cases, s.Runs)
	}
	// 破損率は全実行回数を分母にする（6回中1回）
	if math.Abs(s.BrokenRate-1.0/6) > 1e-9 {
		t.Errorf("破損率 = %v, want %v", s.BrokenRate, 1.0/6)
	}
	if s.BrokenByReason[BrokenTruncated] != 1 {
		t.Errorf("破損理由の内訳 = %v", s.BrokenByReason)
	}
	// 指示遵守率は破損を分母から外す（5回中4回が違反なし）
	if math.Abs(s.ComplianceRate-4.0/5) > 1e-9 {
		t.Errorf("指示遵守率 = %v, want %v", s.ComplianceRate, 4.0/5)
	}
	if s.ViolationCounts["json_not_bare"] != 1 {
		t.Errorf("違反の内訳 = %v", s.ViolationCounts)
	}
	// 再現性: bad σ=0.05, mid σ=0（破損を除くと1件）, good σ=0 → 平均 0.05/3
	if math.Abs(s.MeanScoreStdDev-0.05/3) > 1e-9 {
		t.Errorf("平均σ = %v, want %v", s.MeanScoreStdDev, 0.05/3)
	}
	if math.Abs(s.MaxScoreStdDev-0.05) > 1e-9 {
		t.Errorf("最大σ = %v, want 0.05", s.MaxScoreStdDev)
	}
	// 弁別力: bad(0.25) < mid(0.5) < good(0.8) なので同順位なしの完全一致 = 1
	if math.Abs(s.LabelRankCorrelation-1) > 1e-9 {
		t.Errorf("順位相関 = %v, want 1", s.LabelRankCorrelation)
	}
	if math.Abs(s.MeanScoreByLabel[LabelBad]-0.25) > 1e-9 {
		t.Errorf("badの平均 = %v, want 0.25", s.MeanScoreByLabel[LabelBad])
	}
	// レイテンシは破損した実行も含める（待ち時間は失敗でも発生する）
	if s.LatencyP50MS != 300 || s.LatencyP95MS != 600 {
		t.Errorf("レイテンシ p50/p95 = %d/%d, want 300/600", s.LatencyP50MS, s.LatencyP95MS)
	}
	// コスト: gpt-4o-mini は $0.15/1M入力, $0.60/1M出力。5回×(1000入力+500出力)
	wantCost := 5000*0.15/1_000_000 + 2500*0.60/1_000_000
	if math.Abs(s.TotalCostUSD-wantCost) > 1e-12 {
		t.Errorf("合計コスト = %v, want %v", s.TotalCostUSD, wantCost)
	}
	// 1件あたりは全実行回数（破損含む）で割る。課金は破損した呼び出しにも発生する
	if math.Abs(s.CostPerCallUSD-wantCost/6) > 1e-12 {
		t.Errorf("1件あたりコスト = %v, want %v", s.CostPerCallUSD, wantCost/6)
	}
}

// 破損した実行のスコアを0として混ぜると、破損が多いモデルの弁別力が偶然高く出る。
// 除外していることを固定する。
func TestAggregate破損はスコア集計から除く(t *testing.T) {
	obs := []Observation{
		{CaseID: "c1", Label: LabelGood, Score: 0.9},
		{CaseID: "c1", Label: LabelGood, Broken: true, BrokenReason: BrokenJSONInvalid},
		{CaseID: "c2", Label: LabelBad, Score: 0.1},
		{CaseID: "c2", Label: LabelBad, Broken: true, BrokenReason: BrokenJSONInvalid},
	}
	s := Aggregate(TargetES, "gpt-4o", "", "", obs)
	for _, c := range s.CaseDetails {
		if c.ScoreStdDev != 0 {
			t.Errorf("%s: 破損を除くと1件なのでσは0であるべき: %v", c.CaseID, c.ScoreStdDev)
		}
		if c.BrokenRuns != 1 {
			t.Errorf("%s: 破損回数 = %d, want 1", c.CaseID, c.BrokenRuns)
		}
	}
	if math.Abs(s.MeanScoreByLabel[LabelGood]-0.9) > 1e-9 {
		t.Errorf("goodの平均 = %v, want 0.9（破損を0として混ぜていない）", s.MeanScoreByLabel[LabelGood])
	}
}

// 全実行が破損したケースは平均・σを持たないので、弁別力の系列にも入らない。
func TestAggregate全回破損したケース(t *testing.T) {
	obs := []Observation{
		{CaseID: "c1", Label: LabelGood, Score: 0.9},
		{CaseID: "c2", Label: LabelBad, Broken: true, BrokenReason: BrokenCallFailed},
	}
	s := Aggregate(TargetES, "gpt-4o", "", "", obs)
	if s.BrokenRate != 0.5 {
		t.Errorf("破損率 = %v, want 0.5", s.BrokenRate)
	}
	if _, ok := s.MeanScoreByLabel[LabelBad]; ok {
		t.Error("全回破損したラベルは平均を持たないべき")
	}
	// 系列が1件しか無いので相関は0（例外にしない）
	if s.LabelRankCorrelation != 0 {
		t.Errorf("順位相関 = %v, want 0", s.LabelRankCorrelation)
	}
}

func TestAggregate不安定ケースの検出(t *testing.T) {
	obs := []Observation{
		{CaseID: "stable", Label: LabelGood, Score: 0.80},
		{CaseID: "stable", Label: LabelGood, Score: 0.82},
		{CaseID: "unstable", Label: LabelMid, Score: 0.20},
		{CaseID: "unstable", Label: LabelMid, Score: 0.80},
	}
	s := Aggregate(TargetES, "gpt-4o", "", "", obs)
	if !slices.Contains(s.UnstableCaseIDs, "unstable") {
		t.Errorf("σ>%.2f のケースが検出されていない: %v", UnstableStdDevThreshold, s.UnstableCaseIDs)
	}
	if slices.Contains(s.UnstableCaseIDs, "stable") {
		t.Errorf("安定したケースが混ざっている: %v", s.UnstableCaseIDs)
	}
}

// 通信エラーを破損率とレイテンシに混ぜると数字が壊れる。
// 実測で、ローカルの通信断2回が破損率11.1%、120秒のタイムアウトが p95 120,001ms
// として出てしまった。どちらもモデルの品質とは無関係で、測り直せば消える。
func TestAggregate通信エラーは破損率とレイテンシから除く(t *testing.T) {
	obs := []Observation{
		{CaseID: "c1", Label: LabelGood, Score: 0.8, LatencyMS: 5000},
		{CaseID: "c1", Label: LabelGood, Score: 0.8, LatencyMS: 6000},
		{CaseID: "c2", Label: LabelBad, Score: 0.2, LatencyMS: 5500},
		// 通信エラー（120秒のタイムアウト）
		{CaseID: "c2", Label: LabelBad, LatencyMS: 120001, Broken: true, BrokenReason: BrokenNetwork,
			Error: "context deadline exceeded"},
	}
	s := Aggregate(TargetResume, "gpt-4o-mini", "", "", obs)

	if s.UnmeasuredRuns != 1 {
		t.Errorf("計測できなかった回数 = %d, want 1", s.UnmeasuredRuns)
	}
	// 計測できた3回のうち破損は0回
	if s.BrokenRate != 0 {
		t.Errorf("破損率 = %v, want 0（通信エラーを混ぜていない）", s.BrokenRate)
	}
	if s.BrokenByReason[BrokenNetwork] != 0 {
		t.Errorf("通信エラーが破損の内訳に入っている: %v", s.BrokenByReason)
	}
	// p95 は計測できた3回だけで出す。120001ms を含めると実態と桁が違う
	if s.LatencyP95MS != 6000 {
		t.Errorf("p95 = %dms, want 6000ms（タイムアウトを除外していない）", s.LatencyP95MS)
	}
	// ケース単位の破損回数にも数えない
	for _, c := range s.CaseDetails {
		if c.BrokenRuns != 0 {
			t.Errorf("%s: 破損回数 = %d, want 0", c.CaseID, c.BrokenRuns)
		}
	}
	// スコアの集計からは外れる（通信エラー時のスコアは存在しない）
	if s.MeanScoreByLabel[LabelBad] != 0.2 {
		t.Errorf("badの平均 = %v, want 0.2", s.MeanScoreByLabel[LabelBad])
	}
}

// 応答が返っていない呼び出しにトークン課金は無い。
// ES添削はトークン数を概算で埋めるため、除外しないと実行されなかった
// 呼び出しのコストが合計に混ざる。
func TestAggregate通信エラーはコストに数えない(t *testing.T) {
	obs := []Observation{
		{CaseID: "c1", Label: LabelGood, Score: 0.8, PromptTokens: 1000, CompletionTokens: 500, TokensEstimated: true},
		// ES添削は通信エラーでも概算トークンが入っている
		{CaseID: "c1", Label: LabelGood, PromptTokens: 1000, CompletionTokens: 500, TokensEstimated: true,
			Broken: true, BrokenReason: BrokenNetwork},
	}
	s := Aggregate(TargetES, "gpt-4o-mini", "", "", obs)
	want := 1000*0.15/1_000_000 + 500*0.60/1_000_000
	if math.Abs(s.TotalCostUSD-want) > 1e-12 {
		t.Errorf("合計コスト = %v, want %v（通信エラー分を除外していない）", s.TotalCostUSD, want)
	}
	// 1件あたりも計測できた1回で割る
	if math.Abs(s.CostPerCallUSD-want) > 1e-12 {
		t.Errorf("1件あたりコスト = %v, want %v", s.CostPerCallUSD, want)
	}
}

// 使える出力が返らなかった場合は破損として数える（通信エラーと区別する）。
func TestAggregate応答はあったが使えない場合は破損に数える(t *testing.T) {
	obs := []Observation{
		{CaseID: "c1", Label: LabelGood, Score: 0.8, LatencyMS: 5000},
		{CaseID: "c1", Label: LabelGood, LatencyMS: 5000, Broken: true, BrokenReason: BrokenJSONInvalid},
	}
	s := Aggregate(TargetResume, "gpt-4o-mini", "", "", obs)
	if s.BrokenRate != 0.5 {
		t.Errorf("破損率 = %v, want 0.5", s.BrokenRate)
	}
	if s.UnmeasuredRuns != 0 {
		t.Errorf("計測できなかった回数 = %d, want 0", s.UnmeasuredRuns)
	}
}

func TestAggregate空入力(t *testing.T) {
	s := Aggregate(TargetES, "gpt-4o", "", "", nil)
	if s.BrokenRate != 0 || s.Cases != 0 {
		t.Errorf("空入力で壊れている: %+v", s)
	}
}

func TestDiff(t *testing.T) {
	base := func() *Summary {
		return &Summary{
			Target: TargetES, Model: "gpt-4o", ManifestSHA256: "abc", Cases: 30,
			BrokenRate: 0.10, ComplianceRate: 0.90, LabelRankCorrelation: 0.80,
			MeanScoreStdDev: 0.03, LatencyP95MS: 5000, CostPerCallUSD: 0.001,
		}
	}
	tests := []struct {
		name       string
		mutate     func(*Summary)
		wantMetric string
	}{
		{name: "変化なしなら劣化なし", mutate: func(*Summary) {}},
		{name: "破損率が閾値ぶん増えたら劣化", mutate: func(s *Summary) { s.BrokenRate = 0.15 }, wantMetric: "破損率"},
		{name: "破損率が閾値未満の増加なら劣化ではない", mutate: func(s *Summary) { s.BrokenRate = 0.13 }},
		{name: "破損率が減るのは劣化ではない", mutate: func(s *Summary) { s.BrokenRate = 0.01 }},
		{name: "指示遵守率が下がったら劣化", mutate: func(s *Summary) { s.ComplianceRate = 0.85 }, wantMetric: "指示遵守率"},
		{name: "指示遵守率が上がるのは劣化ではない", mutate: func(s *Summary) { s.ComplianceRate = 1.0 }},
		{name: "順位相関が下がったら劣化", mutate: func(s *Summary) { s.LabelRankCorrelation = 0.70 }, wantMetric: "弁別力(順位相関)"},
		{name: "σが増えたら劣化", mutate: func(s *Summary) { s.MeanScoreStdDev = 0.08 }, wantMetric: "再現性(平均σ)"},
		{name: "p95が1.2倍なら劣化", mutate: func(s *Summary) { s.LatencyP95MS = 6000 }, wantMetric: "レイテンシ(p95)"},
		{name: "p95が1.1倍なら劣化ではない", mutate: func(s *Summary) { s.LatencyP95MS = 5500 }},
		{name: "コストが1.2倍なら劣化", mutate: func(s *Summary) { s.CostPerCallUSD = 0.0012 }, wantMetric: "コスト(1件)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev, cur := base(), base()
			tt.mutate(cur)
			regs, warnings := Diff(prev, cur)
			if len(warnings) != 0 {
				t.Errorf("同一条件なので警告は出ないべき: %v", warnings)
			}
			if tt.wantMetric == "" {
				if len(regs) != 0 {
					t.Errorf("劣化なしのはず: %+v", regs)
				}
				return
			}
			if len(regs) != 1 || regs[0].Metric != tt.wantMetric {
				t.Errorf("劣化 = %+v, want %q 1件", regs, tt.wantMetric)
			}
		})
	}
}

// 入力・モデルが違う結果を比べても意味が無い。警告を出すことを固定する。
func TestDiff前提の違いを警告する(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Summary)
	}{
		{name: "target違い", mutate: func(s *Summary) { s.Target = TargetResume }},
		{name: "マニフェスト違い", mutate: func(s *Summary) { s.ManifestSHA256 = "zzz" }},
		{name: "モデル違い", mutate: func(s *Summary) { s.Model = "gpt-4o-mini" }},
		{name: "件数違い", mutate: func(s *Summary) { s.Cases = 10 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := &Summary{Target: TargetES, Model: "gpt-4o", ManifestSHA256: "abc", Cases: 30}
			cur := &Summary{Target: TargetES, Model: "gpt-4o", ManifestSHA256: "abc", Cases: 30}
			tt.mutate(cur)
			_, warnings := Diff(prev, cur)
			if len(warnings) == 0 {
				t.Error("警告が出ていない")
			}
		})
	}
}
