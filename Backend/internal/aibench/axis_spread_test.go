package aibench

import (
	"bytes"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// 同一入力を複数回採点したときの、採点軸ごとの最大差（元の点）を集計する（SOCAIAGENT-699）。
func TestAggregate採点軸ごとの最大差(t *testing.T) {
	obs := []Observation{
		{CaseID: "a", Label: LabelGood, Score: 0.8, AxisScores: map[string]float64{"star": 6, "spec": 7}},
		{CaseID: "a", Label: LabelGood, Score: 0.8, AxisScores: map[string]float64{"star": 9, "spec": 7}},
		// 破損した実行の軸は混ぜない
		{CaseID: "a", Label: LabelGood, Broken: true, BrokenReason: BrokenSchema, AxisScores: map[string]float64{"spec": 1}},
		{CaseID: "b", Label: LabelBad, Score: 0.2, AxisScores: map[string]float64{"star": 3, "spec": 4}},
		{CaseID: "b", Label: LabelBad, Score: 0.2, AxisScores: map[string]float64{"star": 4, "spec": 5}},
		// 1回しか採点できなかったケースは差を測れないので数えない
		{CaseID: "c", Label: LabelMid, Score: 0.5, AxisScores: map[string]float64{"star": 10, "once": 5}},
	}
	s := Aggregate(TargetES, "gpt-4o", "", "", obs)
	want := map[string]float64{"star": 3, "spec": 1}
	if !maps.Equal(s.AxisMaxSpread, want) {
		t.Errorf("AxisMaxSpread = %v, want %v", s.AxisMaxSpread, want)
	}
	// 閾値を超えた軸は警告として名前を出す（失敗にはしない）
	if !slices.Equal(s.UnstableAxes, []string{"star"}) {
		t.Errorf("UnstableAxes = %v, want [star]", s.UnstableAxes)
	}

	var buf bytes.Buffer
	PrintSummary(&buf, s)
	out := buf.String()
	for _, line := range []string{"ゆらぎ:star", "最大差 3.0点", "ゆらぎ:spec", "最大差 1.0点", "警告"} {
		if !strings.Contains(out, line) {
			t.Errorf("出力に %q が無い:\n%s", line, out)
		}
	}
}

// ES・履歴書の target は軸別の点を残す。残さないと軸ごとの最大差が測れない。
func TestEvaluateは軸別の点を残す(t *testing.T) {
	es := evaluateESResponse(Observation{}, Case{ID: "es", Target: TargetES, Input: Input{ESText: "あ"}}, http.StatusOK,
		[]byte(`{"specificity_score":6,"star_score":7,"company_fit_score":null,"length_balance_score":8,"feedback":"f","improved_text":"i"}`))
	if want := map[string]float64{"specificity_score": 6, "star_score": 7, "length_balance_score": 8}; !maps.Equal(es.AxisScores, want) {
		t.Errorf("ES AxisScores = %v, want %v", es.AxisScores, want)
	}

	text := `{"scores":{"specificity":4,"achievement":3,"role_fit":3,"completeness":4,"readability":3},"summary":"s","items":[]}`
	r := evaluateResumeResponse(Observation{}, Case{ID: "r", Target: TargetResume, Input: Input{ResumeText: "x", CandidateType: "new_grad"}}, CallResult{Text: text})
	if want := map[string]float64{"specificity": 4, "achievement": 3, "role_fit": 3, "completeness": 4, "readability": 3}; !maps.Equal(r.AxisScores, want) {
		t.Errorf("履歴書 AxisScores = %v, want %v", r.AxisScores, want)
	}
}
