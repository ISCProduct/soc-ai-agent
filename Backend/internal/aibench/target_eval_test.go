package aibench

import (
	"errors"
	"math"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// 評価ロジックは LLM を呼ばずに固定する。ここが狂うと「破損していないのに
// 破損として数える」「捏造を見逃す」といった形で、出た数字が判断材料にならない。

func TestEvaluateResumeResponse(t *testing.T) {
	resumeText := "接客のアルバイトで売上を前年比120%に伸ばしました\n研究室でGoのCLIツールを作りました"
	c := Case{ID: "r-001", Label: LabelGood, Target: TargetResume, Input: Input{ResumeText: resumeText}}
	okJSON := `{"score":72,"summary":"要約","items":[{"quote":"接客のアルバイトで売上を前年比120%に伸ばしました","message":"指摘","suggestion":"改善案","severity":"warning"}]}`

	tests := []struct {
		name           string
		res            CallResult
		wantBroken     bool
		wantReason     string
		wantScore      float64
		wantViolations []string
	}{
		{
			name:      "正常",
			res:       CallResult{Text: okJSON},
			wantScore: 0.72,
		},
		{
			name:       "出力上限到達はJSON不正と分けて数える",
			res:        CallResult{Text: `{"score":72,"items":[{"quo`, Truncated: true},
			wantBroken: true, wantReason: BrokenTruncated,
		},
		{
			name:       "API失敗",
			res:        CallResult{Err: errors.New("HTTP 500")},
			wantBroken: true, wantReason: BrokenCallFailed,
		},
		{
			name:       "JSONとして読めない",
			res:        CallResult{Text: "添削できませんでした"},
			wantBroken: true, wantReason: BrokenJSONInvalid,
		},
		{
			name:       "スコアもitemsも無い",
			res:        CallResult{Text: `{"summary":"要約"}`},
			wantBroken: true, wantReason: BrokenSchema,
		},
		{
			// 本番は前後の散文を落として読むので破損ではない。
			// ただし「JSONのみ」という指示には違反しているので別に数える。
			name:           "散文で包まれたJSONは破損ではなく指示違反",
			res:            CallResult{Text: "以下が結果です。\n```json\n" + okJSON + "\n```"},
			wantScore:      0.72,
			wantViolations: []string{"json_not_bare"},
		},
		{
			name: "本文に無い引用は捏造として数える",
			res: CallResult{Text: `{"score":60,"summary":"要約","items":[` +
				`{"quote":"TOEIC900点を取得しました","message":"指摘","suggestion":"改善案","severity":"info"}]}`},
			wantScore:      0.60,
			wantViolations: []string{"quote_not_in_text"},
		},
		{
			// 引用時の改行・空白の入れ直しは捏造ではない
			name: "空白と改行の差は捏造にしない",
			res: CallResult{Text: `{"score":60,"summary":"要約","items":[` +
				`{"quote":"接客のアルバイトで 売上を前年比120%に\n伸ばしました","message":"指摘","suggestion":"改善案","severity":"info"}]}`},
			wantScore: 0.60,
		},
		{
			name: "itemsが上限超え",
			res: CallResult{Text: `{"score":80,"summary":"要約","items":[` +
				strings.Repeat(`{"quote":"研究室でGoのCLIツールを作りました","message":"m","suggestion":"s","severity":"info"},`, 8) +
				`{"quote":"研究室でGoのCLIツールを作りました","message":"m","suggestion":"s","severity":"info"}]}`},
			wantScore:      0.80,
			wantViolations: []string{"items_over_limit"},
		},
		{
			name: "severityが規定外",
			res: CallResult{Text: `{"score":80,"summary":"要約","items":[` +
				`{"quote":"研究室でGoのCLIツールを作りました","message":"m","suggestion":"s","severity":"高"}]}`},
			wantScore:      0.80,
			wantViolations: []string{"severity_invalid"},
		},
		{
			name: "itemsのフィールド欠落",
			res: CallResult{Text: `{"score":80,"summary":"要約","items":[` +
				`{"quote":"研究室でGoのCLIツールを作りました","message":"m","severity":"info"}]}`},
			wantScore:      0.80,
			wantViolations: []string{"item_missing_fields"},
		},
		{
			name:           "summaryが空",
			res:            CallResult{Text: `{"score":80,"summary":"","items":[{"quote":"研究室でGoのCLIツールを作りました","message":"m","suggestion":"s","severity":"info"}]}`},
			wantScore:      0.80,
			wantViolations: []string{"summary_empty"},
		},
		{
			name:           "値域外のスコア",
			res:            CallResult{Text: `{"score":120,"summary":"要約","items":[{"quote":"研究室でGoのCLIツールを作りました","message":"m","suggestion":"s","severity":"info"}]}`},
			wantScore:      1.20,
			wantViolations: []string{"score_out_of_range"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateResumeResponse(Observation{CaseID: c.ID, Label: c.Label}, c, tt.res)
			checkObservation(t, got, tt.wantBroken, tt.wantReason, tt.wantScore, tt.wantViolations)
		})
	}
}

func TestEvaluateInterviewResponse(t *testing.T) {
	transcript := "Interviewer: 学生時代に力を入れたことを教えてください\n" +
		"User: 文化祭の実行委員長として30人のチームをまとめ、来場者を前年の1.5倍にしました\n" +
		"Interviewer: 苦労した点は\n" +
		"User: 意見が割れたときに全員の話を個別に聞いてから決めるようにしました\n"
	c := Case{ID: "i-001", Label: LabelGood, Target: TargetInterviewReport, Input: Input{Transcript: transcript}}

	okBody := func(extra string) string {
		return `{"summary":"よくできました",` +
			`"scores":{"logic":4,"specificity":4,"ownership":4,"communication":4,"enthusiasm":4},` +
			`"evidence":{"logic":"文化祭の実行委員長として30人のチームをまとめ、来場者を前年の1.5倍にしました"},` +
			`"strengths":["主体性","巻き込み力"],"improvements":["結論を先に","数値の補足"],` +
			`"teacher":{"overall_comment":"総評","detailed_evidence":{"logic":"文化祭の実行委員長として30人のチームをまとめました"},"coaching_points":["p1","p2"]}` +
			extra + `}`
	}

	tests := []struct {
		name           string
		res            CallResult
		wantBroken     bool
		wantReason     string
		wantScore      float64
		wantViolations []string
	}{
		{name: "正常", res: CallResult{Text: okBody("")}, wantScore: 0.8},
		{
			name:       "出力上限到達",
			res:        CallResult{Text: `{"summary":"よ`, Truncated: true},
			wantBroken: true, wantReason: BrokenTruncated,
		},
		{
			name:       "scoresが無い",
			res:        CallResult{Text: `{"summary":"よくできました"}`},
			wantBroken: true, wantReason: BrokenSchema,
		},
		{
			// 欠けた項目を無視して平均すると、1項目だけ返すモデルが満点になる
			name: "欠けた評価項目は0として平均する",
			res: CallResult{Text: `{"summary":"s","scores":{"logic":5},"evidence":{},` +
				`"strengths":["a","b"],"improvements":["c","d"],` +
				`"teacher":{"overall_comment":"t","detailed_evidence":{},"coaching_points":["p"]}}`},
			wantScore:      1.0 / 5, // (5+0+0+0+0)/5 = 1.0 → /5 = 0.2
			wantViolations: []string{"scores_invalid"},
		},
		{
			name: "値域外のスコア",
			res: CallResult{Text: `{"summary":"s","scores":{"logic":9,"specificity":4,"ownership":4,"communication":4,"enthusiasm":4},` +
				`"evidence":{},"strengths":["a","b"],"improvements":["c","d"],` +
				`"teacher":{"overall_comment":"t","detailed_evidence":{},"coaching_points":["p"]}}`},
			wantScore:      5.0 / 5, // (9+4+4+4+4)/5 = 5.0 → /5 = 1.0
			wantViolations: []string{"scores_invalid"},
		},
		{
			name: "strengthsが1件（2〜4件の指示に違反）",
			res: CallResult{Text: `{"summary":"s","scores":{"logic":4,"specificity":4,"ownership":4,"communication":4,"enthusiasm":4},` +
				`"evidence":{},"strengths":["a"],"improvements":["c","d"],` +
				`"teacher":{"overall_comment":"t","detailed_evidence":{},"coaching_points":["p"]}}`},
			wantScore:      0.8,
			wantViolations: []string{"strengths_count"},
		},
		{
			name: "teacherが無い",
			res: CallResult{Text: `{"summary":"s","scores":{"logic":4,"specificity":4,"ownership":4,"communication":4,"enthusiasm":4},` +
				`"evidence":{},"strengths":["a","b"],"improvements":["c","d"]}`},
			wantScore:      0.8,
			wantViolations: []string{"teacher_missing"},
		},
		{
			name: "受験者が言っていない根拠は捏造として数える",
			res: CallResult{Text: `{"summary":"s","scores":{"logic":4,"specificity":4,"ownership":4,"communication":4,"enthusiasm":4},` +
				`"evidence":{"logic":"部活で全国大会に出場した経験を語っていました"},` +
				`"strengths":["a","b"],"improvements":["c","d"],` +
				`"teacher":{"overall_comment":"t","detailed_evidence":{},"coaching_points":["p"]}}`},
			wantScore:      0.8,
			wantViolations: []string{"evidence_not_spoken"},
		},
		{
			name: "面接官の発言を根拠にしても捏造として数える",
			res: CallResult{Text: `{"summary":"s","scores":{"logic":4,"specificity":4,"ownership":4,"communication":4,"enthusiasm":4},` +
				`"evidence":{"logic":"学生時代に力を入れたことを教えてください"},` +
				`"strengths":["a","b"],"improvements":["c","d"],` +
				`"teacher":{"overall_comment":"t","detailed_evidence":{},"coaching_points":["p"]}}`},
			wantScore:      0.8,
			wantViolations: []string{"evidence_not_spoken"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateInterviewResponse(Observation{CaseID: c.ID, Label: c.Label}, c, tt.res)
			checkObservation(t, got, tt.wantBroken, tt.wantReason, tt.wantScore, tt.wantViolations)
		})
	}
}

func TestEvaluateESResponse(t *testing.T) {
	esText := strings.Repeat("あ", 400)
	c := Case{ID: "es-001", Label: LabelMid, Target: TargetES, Input: Input{ESText: esText}}
	feedback := strings.Repeat("い", 400)
	improved := strings.Repeat("う", 480) // 元の120%

	body := func(extra string) []byte {
		return []byte(`{"specificity_score":6,"star_score":7,"company_fit_score":null,"length_balance_score":8,` +
			`"feedback":"` + feedback + `","improved_text":"` + improved + `","company_strategy":null` + extra + `}`)
	}

	tests := []struct {
		name           string
		status         int
		body           []byte
		wantBroken     bool
		wantReason     string
		wantScore      float64
		wantViolations []string
	}{
		{
			name: "正常", status: http.StatusOK, body: body(""),
			wantScore: (7.0 - 1) / 9, // (6+7+8)/3 = 7 → 0〜1へ正規化
		},
		{
			// es_review.py は出力上限に到達したときだけ 422 を返す
			name: "422は出力上限到達", status: http.StatusUnprocessableEntity,
			body:       []byte(`{"detail":"添削コメントが長くなりすぎて..."}`),
			wantBroken: true, wantReason: BrokenTruncated,
		},
		{
			name: "500のJSON解析失敗はJSON不正", status: http.StatusInternalServerError,
			body:       []byte(`{"detail":"ES review failed: Expecting value: line 1 column 1 (char 0) json"}`),
			wantBroken: true, wantReason: BrokenJSONInvalid,
		},
		{
			name: "その他の500は呼び出し失敗", status: http.StatusInternalServerError,
			body:       []byte(`{"detail":"ES review failed: Connection error"}`),
			wantBroken: true, wantReason: BrokenCallFailed,
		},
		{
			name: "レスポンスがJSONでない", status: http.StatusOK, body: []byte(`not json`),
			wantBroken: true, wantReason: BrokenJSONInvalid,
		},
		{
			name: "スコアのキーが欠けている", status: http.StatusOK,
			body:       []byte(`{"star_score":7,"length_balance_score":8,"feedback":"a","improved_text":"b"}`),
			wantBroken: true, wantReason: BrokenSchema,
		},
		{
			name: "feedbackが空", status: http.StatusOK,
			body:       []byte(`{"specificity_score":6,"star_score":7,"length_balance_score":8,"feedback":"","improved_text":"b"}`),
			wantBroken: true, wantReason: BrokenSchema,
		},
		{
			name: "feedbackが短すぎる", status: http.StatusOK,
			body: []byte(`{"specificity_score":6,"star_score":7,"length_balance_score":8,` +
				`"feedback":"短い","improved_text":"` + improved + `"}`),
			wantScore:      (7.0 - 1) / 9,
			wantViolations: []string{"feedback_length"},
		},
		{
			name: "改善文が元の文字数を大きく超える", status: http.StatusOK,
			body: []byte(`{"specificity_score":6,"star_score":7,"length_balance_score":8,` +
				`"feedback":"` + feedback + `","improved_text":"` + strings.Repeat("う", 900) + `"}`),
			wantScore:      (7.0 - 1) / 9,
			wantViolations: []string{"improved_text_ratio"},
		},
		{
			// #1524: 企業情報が無いのに企業評価を出してはいけない
			name: "企業名なしで企業適合度を返したら違反", status: http.StatusOK,
			body: []byte(`{"specificity_score":6,"star_score":7,"company_fit_score":8,"length_balance_score":8,` +
				`"feedback":"` + feedback + `","improved_text":"` + improved + `","company_strategy":"対策"}`),
			wantScore:      (7.0 - 1) / 9,
			wantViolations: []string{"company_fit_without_context", "company_strategy_without_context"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateESResponse(Observation{CaseID: c.ID, Label: c.Label}, c, tt.status, tt.body)
			checkObservation(t, got, tt.wantBroken, tt.wantReason, tt.wantScore, tt.wantViolations)
		})
	}
}

// トランスクリプトから役割を復元できないと、面接官の発言を根拠にしても
// 捏造として検出できない。
func TestUtterancesFromTranscript(t *testing.T) {
	transcript := "Interviewer: 質問です\nUser: 回答です\n続きの行\nInterviewer: 次の質問\n"
	got := utterancesFromTranscript(transcript)
	if len(got) != 3 {
		t.Fatalf("発話数 = %d, want 3: %+v", len(got), got)
	}
	if got[0].Role != "ai" || got[1].Role != "user" {
		t.Errorf("役割の復元に失敗: %+v", got)
	}
	if got[1].Text != "回答です\n続きの行" {
		t.Errorf("複数行の発話が繋がっていない: %q", got[1].Text)
	}
}

func TestExtractJSONObject(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		bare bool
	}{
		{name: "素のJSON", in: `{"a":1}`, want: `{"a":1}`, bare: true},
		{name: "前後の空白は素とみなす", in: "  \n" + `{"a":1}` + "\n", want: `{"a":1}`, bare: true},
		{name: "コードフェンス", in: "```json\n" + `{"a":1}` + "\n```", want: `{"a":1}`},
		{name: "散文つき", in: `結果です: {"a":1} 以上`, want: `{"a":1}`},
		{name: "JSONが無ければそのまま", in: `失敗しました`, want: `失敗しました`, bare: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractJSONObject(tt.in); got != tt.want {
				t.Errorf("extractJSONObject = %q, want %q", got, tt.want)
			}
			if got := needsJSONRecovery(tt.in); got == tt.bare {
				t.Errorf("needsJSONRecovery = %v, want %v", got, !tt.bare)
			}
		})
	}
}

// checkObservation は破損・スコア・違反をまとめて検証する。
func checkObservation(t *testing.T, got Observation, wantBroken bool, wantReason string, wantScore float64, wantViolations []string) {
	t.Helper()
	if got.Broken != wantBroken {
		t.Fatalf("Broken = %v (%s: %s), want %v", got.Broken, got.BrokenReason, got.Error, wantBroken)
	}
	if wantBroken {
		if got.BrokenReason != wantReason {
			t.Errorf("BrokenReason = %q, want %q", got.BrokenReason, wantReason)
		}
		return
	}
	if math.Abs(got.Score-wantScore) > 1e-9 {
		t.Errorf("Score = %v, want %v", got.Score, wantScore)
	}
	if len(got.Violations) != len(wantViolations) {
		t.Errorf("Violations = %v, want %v", got.Violations, wantViolations)
		return
	}
	for _, v := range wantViolations {
		if !slices.Contains(got.Violations, v) {
			t.Errorf("違反 %q が検出されていない: %v", v, got.Violations)
		}
	}
}
