package aibench

import (
	"Backend/internal/services/interview"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestLabelTargetScore(t *testing.T) {
	tests := []struct {
		label  string
		want   float64
		wantOK bool
	}{
		{LabelBad, 1.0 / 6, true},
		{LabelMid, 0.5, true},
		{LabelGood, 5.0 / 6, true},
		{"", 0, false},
	}
	for _, tt := range tests {
		got, ok := LabelTargetScore(tt.label)
		if ok != tt.wantOK || !near(got, tt.want) {
			t.Errorf("LabelTargetScore(%q) = %v,%v want %v,%v", tt.label, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestMeanAbsError(t *testing.T) {
	tests := []struct {
		name      string
		want, got []float64
		out       float64
	}{
		{"一致", []float64{1, 2}, []float64{1, 2}, 0},
		{"絶対値（符号が打ち消し合わない）", []float64{1, 3}, []float64{2, 2}, 1},
		{"大小どちらの誤差も数える", []float64{0, 0, 0}, []float64{1, -2, 3}, 2},
		{"長さ違いは0", []float64{1}, []float64{1, 2}, 0},
		{"空は0", nil, nil, 0},
	}
	for _, tt := range tests {
		if got := MeanAbsError(tt.want, tt.got); !near(got, tt.out) {
			t.Errorf("%s: %v want %v", tt.name, got, tt.out)
		}
	}
}

func TestPearsonCorrelation(t *testing.T) {
	tests := []struct {
		name   string
		xs, ys []float64
		want   float64
	}{
		{"完全な正の線形", []float64{1, 2, 3}, []float64{2, 4, 6}, 1},
		{"完全な負の線形", []float64{1, 2, 3}, []float64{6, 4, 2}, -1},
		{"定数は0", []float64{1, 1, 1}, []float64{1, 2, 3}, 0},
		{"1件は0", []float64{1}, []float64{1}, 0},
		{"線形でない単調増加は1未満", []float64{1, 2, 3}, []float64{1, 2, 10}, 9 / math.Sqrt(2*48.666666666666664)},
	}
	for _, tt := range tests {
		if got := PearsonCorrelation(tt.xs, tt.ys); !near(got, tt.want) {
			t.Errorf("%s: %v want %v", tt.name, got, tt.want)
		}
	}
}

func TestFairnessGaps(t *testing.T) {
	tests := []struct {
		name         string
		in           map[string][]float64
		n            int
		mean, maxGap float64
	}{
		{"差なし", map[string][]float64{"a": {0.5, 0.5}}, 1, 0, 0},
		{"組内の最大-最小", map[string][]float64{"a": {0.5, 0.7}, "b": {0.6, 0.6, 0.9}}, 2, (0.2 + 0.3) / 2, 0.3},
		{"相手が居ない組は数えない", map[string][]float64{"a": {0.5}, "b": {0.2, 0.4}}, 1, 0.2, 0.2},
		{"空", nil, 0, 0, 0},
	}
	for _, tt := range tests {
		n, mean, mx := FairnessGaps(tt.in)
		if n != tt.n || !near(mean, tt.mean) || !near(mx, tt.maxGap) {
			t.Errorf("%s: n=%d mean=%v max=%v want %d %v %v", tt.name, n, mean, mx, tt.n, tt.mean, tt.maxGap)
		}
	}
}

func ivObs(id, label string, score float64, axis map[string]float64) Observation {
	return Observation{CaseID: id, Label: label, Score: score, AxisScores: axis}
}

// 3ケース（bad/mid/good）の採点がラベルの中央値と全く同じなら MAE は 0・相関は 1。
func TestAggregate_人手ラベルMAEと相関(t *testing.T) {
	obs := []Observation{ivObs("b", LabelBad, 1.0/6, nil), ivObs("m", LabelMid, 0.5, nil), ivObs("g", LabelGood, 5.0/6, nil)}
	s := Aggregate(TargetInterviewReport, "m", "e", "t", obs)
	if s.ValidityCases != 3 || !near(s.LabelMAE, 0) || !near(s.LabelPearson, 1) {
		t.Errorf("一致: %+v", s)
	}
	// 全件を 0.2 高く付けると MAE は 0.2。相関は 1 のまま（較正のずれは MAE だけが見る）。
	for i := range obs {
		obs[i].Score += 0.1
	}
	obs[0].Score += 0.3 // bad だけ +0.4
	s = Aggregate(TargetInterviewReport, "m", "e", "t", obs)
	if want := (0.4 + 0.1 + 0.1) / 3; !near(s.LabelMAE, want) {
		t.Errorf("MAE = %v want %v", s.LabelMAE, want)
	}
	// 面接レポート以外では計測しない
	if s := Aggregate(TargetResume, "m", "e", "t", obs); s.ValidityCases != 0 {
		t.Errorf("他 target で計測された: %d", s.ValidityCases)
	}
}

func TestAggregate_軸別MAEはラベルのあるケースだけ(t *testing.T) {
	a := ivObs("a", LabelGood, 0.8, map[string]float64{"logic": 4, "ownership": 3})
	a.AxisLabels = map[string]float64{"logic": 5, "ownership": 3}
	b := ivObs("b", LabelBad, 0.2, map[string]float64{"logic": 1, "ownership": 1})
	b.AxisLabels = map[string]float64{"logic": 3}
	c := ivObs("c", LabelMid, 0.5, map[string]float64{"logic": 5}) // 軸ラベル無し→スキップ
	s := Aggregate(TargetInterviewReport, "m", "e", "t", []Observation{a, b, c})
	if !near(s.AxisMAE["logic"], 1.5) || s.AxisMAECases["logic"] != 2 { // (|4-5|+|1-3|)/2
		t.Errorf("logic: %v %v", s.AxisMAE, s.AxisMAECases)
	}
	if !near(s.AxisMAE["ownership"], 0) || s.AxisMAECases["ownership"] != 1 {
		t.Errorf("ownership: %v %v", s.AxisMAE, s.AxisMAECases)
	}
	// 軸ラベルが1件も無ければ出さない
	if s := Aggregate(TargetInterviewReport, "m", "e", "t", []Observation{c}); s.AxisMAE != nil {
		t.Errorf("軸ラベル無しで AxisMAE が出た: %v", s.AxisMAE)
	}
}

func TestAggregate_公平性ペア差(t *testing.T) {
	mk := func(id, pair, variant string, score float64) Observation {
		o := ivObs(id, LabelMid, score, nil)
		o.FairnessPair, o.ContextVariant = pair, variant
		return o
	}
	obs := []Observation{
		mk("a1", "p1", "neutral", 0.60), mk("a2", "p1", "charged", 0.50), // 差 0.10
		mk("b1", "p2", "neutral", 0.40), mk("b2", "p2", "charged", 0.40), // 差 0
		ivObs("x", LabelMid, 0.5, nil), // ペア無しは数えない
	}
	s := Aggregate(TargetInterviewReport, "m", "e", "t", obs)
	if s.FairnessPairs != 2 || !near(s.FairnessMeanGap, 0.05) || !near(s.FairnessMaxGap, 0.10) {
		t.Errorf("公平性: %d %v %v", s.FairnessPairs, s.FairnessMeanGap, s.FairnessMaxGap)
	}
}

func hasMetric(regs []Regression, m string) bool {
	for _, r := range regs {
		if r.Metric == m {
			return true
		}
	}
	return false
}

func TestDiff_妥当性と公平性のしきい値(t *testing.T) {
	base := func() *Summary {
		return &Summary{Target: TargetInterviewReport, Model: "m", Cases: 1, MeasuredRuns: 1,
			ValidityCases: 10, LabelMAE: 0.20, AxisMAE: map[string]float64{"logic": 1.0, "ownership": 1.0},
			FairnessPairs: 4, FairnessMeanGap: 0.02, FairnessMaxGap: 0.05}
	}
	tests := []struct {
		name   string
		mut    func(*Summary)
		metric string
		want   bool
	}{
		{"MAEが閾値ちょうど悪化", func(s *Summary) { s.LabelMAE = 0.25 }, "人手ラベルMAE", true},
		{"MAEが閾値未満の悪化", func(s *Summary) { s.LabelMAE = 0.24 }, "人手ラベルMAE", false},
		{"MAEの改善は劣化でない", func(s *Summary) { s.LabelMAE = 0.10 }, "人手ラベルMAE", false},
		{"軸別MAE平均が0.25点悪化", func(s *Summary) { s.AxisMAE = map[string]float64{"logic": 1.5, "ownership": 1.0} }, "軸別MAE(平均)", true},
		{"軸別MAE平均が閾値未満", func(s *Summary) { s.AxisMAE = map[string]float64{"logic": 1.4, "ownership": 1.0} }, "軸別MAE(平均)", false},
		{"公平性の平均差が悪化", func(s *Summary) { s.FairnessMeanGap = 0.07 }, "公平性(平均差)", true},
		{"公平性の最大差が上限超え", func(s *Summary) { s.FairnessMaxGap = 0.11 }, "公平性(最大差)", true},
		{"公平性の最大差が上限以内", func(s *Summary) { s.FairnessMaxGap = 0.10 }, "公平性(最大差)", false},
		{"前回が未計測なら比べない", func(s *Summary) { s.LabelMAE = 0.9 }, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev, cur := base(), base()
			tt.mut(cur)
			if tt.metric == "" {
				prev.ValidityCases = 0 // 旧 JSON
				prev.LabelMAE = 0
				regs, _ := Diff(prev, cur)
				if hasMetric(regs, "人手ラベルMAE") {
					t.Errorf("未計測の前回と比べた: %+v", regs)
				}
				return
			}
			regs, _ := Diff(prev, cur)
			if got := hasMetric(regs, tt.metric); got != tt.want {
				t.Errorf("%s 検知=%v want %v (%+v)", tt.metric, got, tt.want, regs)
			}
		})
	}
}

func TestLoadManifest_軸別ラベルと公平性ペアの検証(t *testing.T) {
	line := func(extra string) string {
		return `{"id":"a","target":"interview-report","label":"good",` + extra + `"input":{"transcript":"User: はい"}}`
	}
	tests := []struct {
		name, extra, wantErr string
	}{
		{"軸別ラベルあり", `"axis_labels":{"logic":4,"enthusiasm":0},`, ""},
		{"ルーブリックに無い軸", `"axis_labels":{"charisma":3},`, "ルーブリックの軸ではない"},
		{"値域外", `"axis_labels":{"logic":6},`, "0〜"},
		{"ペアだけ指定", `"fairness_pair":"p",`, "両方必要"},
		{"ペアと文脈", `"fairness_pair":"p","context_variant":"neutral",`, ""},
	}
	for _, tt := range tests {
		_, err := LoadManifest(writeManifest(t, line(tt.extra)))
		if tt.wantErr == "" && err != nil || tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
			t.Errorf("%s: err=%v want %q", tt.name, err, tt.wantErr)
		}
	}
}

// 公平性ケースは「回答が同一で企業文脈だけ違う」ことが前提。ここが崩れると
// ペア間のスコア差が企業文脈の影響ではなくなる。
func TestFairnessManifest_回答は同一で企業文脈だけが違う(t *testing.T) {
	cases, err := LoadManifest("../../../docs/research/ai-eval/interview-report-fairness.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	userLines := func(tr string) string {
		var b strings.Builder
		for _, u := range utterancesFromTranscript(tr) {
			if u.Role == "user" {
				b.WriteString(u.Text + "\n")
			}
		}
		return b.String()
	}
	pairs := map[string]map[string]Case{}
	for _, c := range cases {
		if pairs[c.FairnessPair] == nil {
			pairs[c.FairnessPair] = map[string]Case{}
		}
		pairs[c.FairnessPair][c.ContextVariant] = c
	}
	if len(pairs) < 4 {
		t.Errorf("ペア数 = %d, want >= 4", len(pairs))
	}
	for id, p := range pairs {
		n, ch := p["neutral"], p["charged"]
		if n.ID == "" || ch.ID == "" {
			t.Errorf("%s: neutral と charged が揃っていない", id)
			continue
		}
		if userLines(n.Input.Transcript) != userLines(ch.Input.Transcript) {
			t.Errorf("%s: 受験者の回答が同一でない", id)
		}
		if n.Input.Transcript == ch.Input.Transcript || n.Label != ch.Label {
			t.Errorf("%s: 企業文脈が違う・ラベルが同じ、のどちらかが崩れている", id)
		}
	}
}

// 面接レポートは本番と同じ strict JSON Schema で呼ぶ（JSON mode ではない）。
func TestInterviewTarget_strictJSONSchemaで呼ぶ(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &payload)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()
	t.Setenv("OPENAI_BASE_URL", srv.URL)
	t.Setenv("OPENAI_API_KEY", "テスト用")

	newInterviewTarget("gpt-4o-mini").Run(context.Background(), Case{ID: "x", Label: LabelMid, Target: TargetInterviewReport,
		Input: Input{Transcript: "Interviewer: 質問\nUser: 回答\n"}})

	rf, _ := payload["response_format"].(map[string]any)
	js, _ := rf["json_schema"].(map[string]any)
	if rf["type"] != "json_schema" || js["strict"] != true || js["name"] != "interview_report" {
		t.Errorf("response_format = %+v", rf)
	}
	// 再現性の計測が本番と同じ条件になるよう、温度・seed・出力上限も本番の定数を送る。
	if payload["seed"] != float64(interview.ReportSeed) || payload["max_completion_tokens"] != float64(interview.ReportMaxTokens) ||
		payload["temperature"] != float64(interview.ReportTemperature) {
		t.Errorf("seed=%v max_completion_tokens=%v temperature=%v（本番の定数と違う）", payload["seed"], payload["max_completion_tokens"], payload["temperature"])
	}
	msgs, _ := payload["messages"].([]any)
	user, _ := msgs[len(msgs)-1].(map[string]any)
	if !strings.Contains(user["content"].(string), "User [utterance_id=2]: 回答") {
		t.Errorf("本番と同じ utterance_id 付きの発話がプロンプトに無い: %v", user["content"])
	}
}
