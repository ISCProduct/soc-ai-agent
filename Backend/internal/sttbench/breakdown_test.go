package sttbench

import "testing"

func condKey(r CaseResult) string   { return r.Condition }
func sourceKey(r CaseResult) string { return r.Source }

// TestBreakdown_SeparatesConditions は #1484 の核心。
// 母数の小さい条件が全体平均に埋もれず、条件別に出せることを確認する。
func TestBreakdown_SeparatesConditions(t *testing.T) {
	results := []CaseResult{
		{ID: "a", Condition: "clean", CER: 0.01, NumbersMatched: 2, NumbersTotal: 2},
		{ID: "b", Condition: "clean", CER: 0.03, NumbersMatched: 1, NumbersTotal: 1},
		{ID: "c", Condition: "noisy", CER: 0.30, NumbersMatched: 0, NumbersTotal: 2},
	}
	got := Breakdown(results, condKey)
	if len(got) != 2 {
		t.Fatalf("グループ数 = %d, want 2", len(got))
	}
	m := map[string]GroupSummary{}
	for _, g := range got {
		m[g.Key] = g
	}
	if m["clean"].Cases != 2 {
		t.Errorf("clean 件数 = %d, want 2", m["clean"].Cases)
	}
	if m["clean"].MeanCER < 0.019 || m["clean"].MeanCER > 0.021 {
		t.Errorf("clean 平均CER = %.3f, want ~0.02", m["clean"].MeanCER)
	}
	// noisy が clean と分かれて出ること（埋もれない）
	if m["noisy"].MeanCER < 0.29 {
		t.Errorf("noisy 平均CER = %.3f, want ~0.30（埋もれている）", m["noisy"].MeanCER)
	}
	// 数値正解率: clean 3/3=100%, noisy 0/2=0%
	if m["clean"].NumberAccuracy != 1.0 {
		t.Errorf("clean 数値正解率 = %.2f, want 1.0", m["clean"].NumberAccuracy)
	}
	if m["noisy"].NumberAccuracy != 0.0 {
		t.Errorf("noisy 数値正解率 = %.2f, want 0.0", m["noisy"].NumberAccuracy)
	}
}

// 失敗ケースは平均CERに含めず、失敗率の母数には数える。
func TestBreakdown_FailuresExcludedFromMeanButCounted(t *testing.T) {
	results := []CaseResult{
		{ID: "a", Condition: "x", CER: 0.02},
		{ID: "b", Condition: "x", Failed: true, CER: 0.99}, // 失敗はCERを平均に入れない
	}
	g := Breakdown(results, condKey)[0]
	if g.Cases != 2 {
		t.Errorf("件数 = %d, want 2", g.Cases)
	}
	if g.MeanCER < 0.019 || g.MeanCER > 0.021 {
		t.Errorf("平均CER = %.3f, want ~0.02（失敗を除く）", g.MeanCER)
	}
	if g.FailureRate != 0.5 {
		t.Errorf("失敗率 = %.2f, want 0.5", g.FailureRate)
	}
}

// 母数の無い指標は 0（全滅）ではなく -1（母数なし）で返す。
func TestBreakdown_MissingVsZero(t *testing.T) {
	// 数値チェックが1件も無いケース群
	noNumbers := Breakdown([]CaseResult{{ID: "a", Condition: "x"}}, condKey)[0]
	if noNumbers.NumberAccuracy != -1 {
		t.Errorf("数値なし = %.0f, want -1（母数なしと全滅を区別）", noNumbers.NumberAccuracy)
	}
	// 数値はあるが全て外した = 0.0（母数ありの全滅）
	allWrong := Breakdown([]CaseResult{
		{ID: "a", Condition: "x", NumbersMatched: 0, NumbersTotal: 3},
	}, condKey)[0]
	if allWrong.NumberAccuracy != 0.0 {
		t.Errorf("全滅 = %.2f, want 0.0", allWrong.NumberAccuracy)
	}
}

// 由来別（合成/実発話）でも同じロジックが効く。
func TestBreakdown_BySource(t *testing.T) {
	results := []CaseResult{
		{ID: "a", Source: "synthetic", CER: 0.02},
		{ID: "b", Source: "real", CER: 0.15},
		{ID: "c", Source: "real", CER: 0.25},
	}
	m := map[string]GroupSummary{}
	for _, g := range Breakdown(results, sourceKey) {
		m[g.Key] = g
	}
	if m["synthetic"].Cases != 1 || m["real"].Cases != 2 {
		t.Fatalf("由来別の件数が誤り: %+v", m)
	}
	// 実発話のほうが CER が高いことが分かる（合成だけで判断しない根拠）
	if !(m["real"].MeanCER > m["synthetic"].MeanCER) {
		t.Errorf("real(%.3f) が synthetic(%.3f) より高くない", m["real"].MeanCER, m["synthetic"].MeanCER)
	}
}

func TestFilterByCondition(t *testing.T) {
	cases := []Case{
		{ID: "a", ReferenceText: "x", Condition: "clean"},
		{ID: "b", ReferenceText: "x", Condition: "noisy"},
		{ID: "c", ReferenceText: "x"}, // 未指定 -> unspecified
	}
	if got := FilterByCondition(cases, ""); len(got) != 3 {
		t.Errorf("空指定は全件: %d", len(got))
	}
	if got := FilterByCondition(cases, "noisy"); len(got) != 1 || got[0].ID != "b" {
		t.Errorf("noisy 絞り込み失敗: %+v", got)
	}
	if got := FilterByCondition(cases, "unspecified"); len(got) != 1 || got[0].ID != "c" {
		t.Errorf("未指定の条件で c を拾えない: %+v", got)
	}
}

// ConditionOf / SourceOf の既定値。
func TestCaseDefaults(t *testing.T) {
	c := Case{ID: "a", ReferenceText: "x"}
	if c.ConditionOf() != "unspecified" {
		t.Errorf("条件既定 = %q, want unspecified", c.ConditionOf())
	}
	if c.SourceOf() != "synthetic" {
		t.Errorf("由来既定 = %q, want synthetic（既存フィクスチャは合成）", c.SourceOf())
	}
}
