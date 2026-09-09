package interview

import (
	"strings"
	"testing"
)

// 完了定義: S/T/A/R それぞれの欠落を判定できること。
func TestAnalyzeSTAR_DetectsMissingElement(t *testing.T) {
	tests := []struct {
		name    string
		answer  string
		missing STARElement
		present []STARElement
	}{
		{
			name:    "成果(R)が抜けた回答",
			answer:  "大学のチーム開発で、進捗が遅れているという課題がありました。私はタスク管理の仕組みを提案して導入しました。",
			missing: STARResult,
			present: []STARElement{STARSituation, STARTask, STARAction},
		},
		{
			name:    "行動(A)が抜けた回答",
			answer:  "アルバイト先で客からのクレームが問題になっていて、最終的に件数が30件から5件まで減りました。",
			missing: STARAction,
			present: []STARElement{STARSituation, STARTask, STARResult},
		},
		{
			name:    "状況(S)が抜けた回答",
			answer:  "在庫の管理が追いつかないという課題に対して、集計を自動化する仕組みを実装し、作業時間を半分に短縮しました。",
			missing: STARSituation,
			present: []STARElement{STARTask, STARAction, STARResult},
		},
		{
			name:    "課題(T)が抜けた回答",
			answer:  "研究室で画像認識のモデルを実装し、精度を80%まで向上させました。",
			missing: STARTask,
			present: []STARElement{STARSituation, STARAction, STARResult},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := AnalyzeSTAR(tt.answer)
			if a.Has(tt.missing) {
				t.Errorf("%s を含むと誤判定した（根拠: %q）", STARLabel(tt.missing), a.Evidence[tt.missing])
			}
			for _, e := range tt.present {
				if !a.Has(e) {
					t.Errorf("%s を検出できていない", STARLabel(e))
				}
			}
		})
	}
}

// 判定には根拠となる語が伴うこと。根拠が無い判定は検証できない。
func TestAnalyzeSTAR_ReturnsEvidence(t *testing.T) {
	const answer = "研究室で画像認識のモデルを実装し、精度を80%まで向上させました。"
	a := AnalyzeSTAR(answer)
	for _, e := range []STARElement{STARSituation, STARAction, STARResult} {
		ev := a.Evidence[e]
		if ev == "" {
			t.Errorf("%s の根拠が空", STARLabel(e))
			continue
		}
		// 根拠は回答から抜き出した実際の文字列でなければ、判定を検証できない
		if !strings.Contains(answer, ev) {
			t.Errorf("%s の根拠 %q が回答に含まれない", STARLabel(e), ev)
		}
	}
}

// 完了定義: 成果(R)が抜けた回答に、成果を促す質問が返ること。
func TestSTARFollowUpQuestion_AsksForMissingResult(t *testing.T) {
	answer := "大学のチーム開発で、進捗が遅れているという課題がありました。私はタスク管理の仕組みを提案して導入しました。"
	q, element := STARFollowUpQuestion(answer)
	if element != STARResult {
		t.Fatalf("欠落要素 = %q, want R", element)
	}
	if !strings.Contains(q, "結果") {
		t.Errorf("成果を促す質問になっていない: %q", q)
	}
}

// 欠落要素ごとに異なる質問が返ること。
// 全て同じ文言なら、欠落を判定している意味がない。
func TestSTARFollowUpQuestion_DiffersPerElement(t *testing.T) {
	answers := map[STARElement]string{
		STARResult:    "大学のチーム開発で課題があり、仕組みを提案して導入しました。",
		STARAction:    "アルバイト先でクレームが問題になっていて、件数が30件から5件に減りました。",
		STARSituation: "在庫管理が追いつかない課題に対し、集計を自動化する仕組みを実装し半分に短縮しました。",
		STARTask:      "研究室で画像認識のモデルを実装し、精度を80%まで向上させました。",
	}
	seen := map[string]STARElement{}
	for want, answer := range answers {
		q, got := STARFollowUpQuestion(answer)
		if got != want {
			t.Errorf("%s の欠落を狙うはずが %s になった", STARLabel(want), STARLabel(got))
			continue
		}
		if prev, dup := seen[q]; dup {
			t.Errorf("%s と %s で質問文が同じ: %q", STARLabel(prev), STARLabel(want), q)
		}
		seen[q] = want
	}
}

// STAR が揃った回答は深掘りしない。
func TestSTARFollowUpQuestion_CompleteAnswerNeedsNothing(t *testing.T) {
	answer := "大学の研究室で、実験データの集計が手作業で追いつかないという課題がありました。" +
		"私は集計スクリプトを実装し、作業時間を週5時間から1時間へ短縮しました。"
	q, element := STARFollowUpQuestion(answer)
	if q != "" || element != "" {
		t.Errorf("STARが揃っているのに深掘りしようとした: %q (%s)", q, element)
	}
}

// 空の回答で落ちないこと。全要素が欠落として返る。
func TestAnalyzeSTAR_EmptyAnswer(t *testing.T) {
	a := AnalyzeSTAR("   ")
	if len(a.Missing()) != 4 {
		t.Errorf("欠落要素 = %v, want 4件", a.Missing())
	}
	q, element := STARFollowUpQuestion("")
	if q == "" || element != STARResult {
		t.Errorf("空回答で質問が返らない: %q (%s)", q, element)
	}
}

// 深掘りは成果(R)を最優先にする。
// 複数欠けているときに状況や課題から聞くと、成果まで辿り着かずに時間切れになる。
func TestSTARMissing_PrioritizesResult(t *testing.T) {
	a := AnalyzeSTAR("がんばりました。")
	missing := a.Missing()
	if len(missing) == 0 || missing[0] != STARResult {
		t.Errorf("優先順 = %v, want R が先頭", missing)
	}
}

// 欠落要素をLLMプロンプトに明示していること。
// これが抜けると、成果が語られていない回答にも「きっかけ」を聞き返す。
func TestBuildFollowUpUserPrompt_NamesMissingElement(t *testing.T) {
	// 成果(R)だけが無い回答
	answer := "大学のチーム開発で進捗の遅れという課題があり、タスク管理の仕組みを提案して導入しました。"
	got := buildFollowUpUserPrompt("学生時代に力を入れたことは？", answer)

	if !strings.Contains(got, answer) {
		t.Error("回答本文がプロンプトに含まれていない")
	}
	if !strings.Contains(got, "「成果」") {
		t.Errorf("欠落要素を指定していない: %s", got)
	}
	for _, other := range []string{"「状況」", "「課題」", "「行動」"} {
		if strings.Contains(got, other) {
			t.Errorf("欠けていない要素 %s を指定している: %s", other, got)
		}
	}
}

// STAR が揃った回答には余計な指定をしない。
func TestBuildFollowUpUserPrompt_CompleteAnswerHasNoDirective(t *testing.T) {
	answer := "大学のゼミで集計が遅いという課題があり、自動化を実装して作業時間を5時間短縮しました。"
	got := buildFollowUpUserPrompt("学生時代に力を入れたことは？", answer)
	if strings.Contains(got, "含まれていません") {
		t.Errorf("STARが揃っているのに欠落を指定した: %s", got)
	}
}
