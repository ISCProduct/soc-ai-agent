package interview

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/models"
	"Backend/internal/openai"
)

// evidenceTestUtterances は生成テスト用の面接ログ。
func evidenceTestUtterances() []models.InterviewUtterance {
	return []models.InterviewUtterance{
		{Role: "ai", Text: "学生時代に力を入れたことを教えてください。"},
		{Role: "user", Text: "はい、私は大学時代に軽音サークルの代表を務めていました。入学した当初は部員が8人しかいなくて、このままだと廃部になるという話が出ていました。"},
		{Role: "user", Text: "そこで私が新歓ライブの企画を提案して、SNSでの告知を担当しました。結果として、翌年の新入部員は23人まで増えて、部員数を3倍にすることができました。"},
	}
}

// 実発話に基づく evidence（学生向け）。
func genuineEvidence() map[string]string {
	return map[string]string{
		"logic":         "結果として、翌年の新入部員は23人まで増えて、部員数を3倍にすることができました。",
		"specificity":   "入学した当初は部員が8人しかいなくて",
		"ownership":     "そこで私が新歓ライブの企画を提案して、SNSでの告知を担当しました。",
		"communication": "新歓ライブを企画してSNS告知を担当し、部員数を3倍にした",
		"enthusiasm":    "私は大学時代に軽音サークルの代表を務めていました",
	}
}

// 実発話に基づく detailed_evidence（教員向け。引用＋指導ポイント）。
func genuineTeacherEvidence() map[string]string {
	return map[string]string{
		"logic":     "「結果として、翌年の新入部員は23人まで増えて、部員数を3倍にすることができました」と数値で成果を語れている。次は自分の役割をより具体的に言語化させたい。",
		"ownership": "「私が新歓ライブの企画を提案して、SNSでの告知を担当しました」と主体性が明確。ただし他メンバーとの分担に触れていないため、そこを掘る指導が有効。",
	}
}

// fabricated は実発話のどこにも無い根拠。
const fabricated = "TOEICで900点を取得し、英語での商談経験もあります。"

// reportJSON はレポート1件分の LLM 応答を組み立てる。
func reportJSON(t *testing.T, evidence, teacherEvidence map[string]string) string {
	t.Helper()
	body, err := json.Marshal(reportPayload{
		Summary:      "落ち着いて回答できていました。",
		Scores:       map[string]int{"logic": 3, "specificity": 2, "ownership": 4, "communication": 3, "enthusiasm": 4},
		Evidence:     evidence,
		Strengths:    []string{"結論から話せる"},
		Improvements: []string{"数値を添える"},
		Teacher: &teacherReport{
			OverallComment:   "指導しやすい",
			DetailedEvidence: teacherEvidence,
			CoachingPoints:   []string{"数値を促す"},
		},
	})
	if err != nil {
		t.Fatalf("テスト用JSONの組み立てに失敗: %v", err)
	}
	return string(body)
}

// chatStub は ChatCompletionJSON の応答を順番に返し、呼ばれた回数を数える。
// 応答を使い切ったら最後の応答を返し続ける。
func chatStub(t *testing.T, responses ...string) (*openai.Client, func() int) {
	t.Helper()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := responses[min(calls, len(responses)-1)]
		calls++
		w.Header().Set("Content-Type", "application/json")
		resp, _ := json.Marshal(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": content}}},
		})
		w.Write(resp)
	}))
	t.Cleanup(server.Close)
	return openai.NewWithBaseURL(server.URL, "gpt-4o-mini"), func() int { return calls }
}

// runReport はスタブを組んでレポートを1件生成する。
func runReport(t *testing.T, utterances []models.InterviewUtterance, responses ...string) (*models.InterviewReport, func() int) {
	t.Helper()
	const sessionID = uint(7)
	sessionRepo := newSessionRepoStub(&models.InterviewSession{ID: sessionID, UserID: 3, Status: "finished", Language: "ja"})
	reportRepo := &reportRepoStub{}
	client, calls := chatStub(t, responses...)
	svc := NewInterviewService(sessionRepo, &utterRepoStub{utterances: utterances}, reportRepo, nil, nil, client, nil)

	if err := svc.generateReport(context.Background(), sessionID); err != nil {
		t.Fatalf("レポート生成が落ちた: %v", err)
	}
	if reportRepo.upsertCalls != 1 {
		t.Fatalf("Upsert=%d want 1: レポートが保存されていない", reportRepo.upsertCalls)
	}
	return reportRepo.last, calls
}

// savedEvidence は保存された学生向け evidence を読む。
func savedEvidence(t *testing.T, report *models.InterviewReport) map[string]string {
	t.Helper()
	var got map[string]string
	if report.EvidenceJSON == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(report.EvidenceJSON), &got); err != nil {
		t.Fatalf("evidence_json が読めない (%q): %v", report.EvidenceJSON, err)
	}
	return got
}

// savedTeacherEvidence は保存された教員向け detailed_evidence を読む。
func savedTeacherEvidence(t *testing.T, report *models.InterviewReport) map[string]string {
	t.Helper()
	var got teacherReport
	if err := json.Unmarshal([]byte(report.TeacherReportJSON), &got); err != nil {
		t.Fatalf("teacher_report_json が読めない (%q): %v", report.TeacherReportJSON, err)
	}
	return got.DetailedEvidence
}

// TestGenerateReport_EvidenceVerification は #1527 の中心的な回帰テスト。
//
// evidence は自由記述なのでスキーマ検証では捏造を検出できない。
// 学生が言っていない発言が「根拠」として学生向け・教員向けの両レポートに出ると、
// レポートを前提に指導する教員の信頼を直接損なう。
// 実発話と照合して、照合できない項目は再生成 → それでも駄目なら空にして保存する。
func TestGenerateReport_EvidenceVerification(t *testing.T) {
	t.Parallel()

	withKey := func(m map[string]string, key, val string) map[string]string {
		out := maps.Clone(m)
		out[key] = val
		return out
	}

	tests := []struct {
		name         string
		utterances   []models.InterviewUtterance
		responses    []string
		wantCalls    int
		wantEvidence map[string]string
		wantTeacher  map[string]string
		why          string
	}{
		{
			name:         "全項目が実発話に基づくなら1回で確定する",
			utterances:   evidenceTestUtterances(),
			responses:    []string{reportJSON(t, genuineEvidence(), genuineTeacherEvidence())},
			wantCalls:    1,
			wantEvidence: genuineEvidence(),
			wantTeacher:  genuineTeacherEvidence(),
			why:          "照合できているのに再生成している（LLM費用が二重に掛かる）",
		},
		{
			name:       "学生向けに捏造があれば作り直し、2回目が通ればそれを保存する",
			utterances: evidenceTestUtterances(),
			responses: []string{
				reportJSON(t, withKey(genuineEvidence(), "specificity", fabricated), genuineTeacherEvidence()),
				reportJSON(t, genuineEvidence(), genuineTeacherEvidence()),
			},
			wantCalls:    2,
			wantEvidence: genuineEvidence(),
			wantTeacher:  genuineTeacherEvidence(),
			why:          "捏造された根拠をそのまま保存している",
		},
		{
			name:       "作り直しても捏造なら、その項目だけ空にして保存する",
			utterances: evidenceTestUtterances(),
			responses: []string{
				reportJSON(t, withKey(genuineEvidence(), "specificity", fabricated), genuineTeacherEvidence()),
			},
			wantCalls:    reportGenerationAttempts,
			wantEvidence: withKey(genuineEvidence(), "specificity", ""),
			wantTeacher:  genuineTeacherEvidence(),
			why:          "捏造された根拠が残っている、または照合できた項目まで捨てている",
		},
		{
			// 指摘1: 教員向けだけ無検証だと、指導の前提になるレポートに捏造が残る
			name:       "教員向けの根拠が捏造なら、その項目だけ空にして保存する",
			utterances: evidenceTestUtterances(),
			responses: []string{
				reportJSON(t, genuineEvidence(), withKey(genuineTeacherEvidence(), "specificity",
					"「"+fabricated+"」と述べており、語学力の裏付けがある。実務での活用場面を聞き出したい。")),
			},
			wantCalls:    reportGenerationAttempts,
			wantEvidence: genuineEvidence(),
			wantTeacher:  withKey(genuineTeacherEvidence(), "specificity", ""),
			why:          "教員向けレポートに捏造された根拠が残っている",
		},
		{
			name:       "教員向けの捏造でも作り直しが走り、2回目が通ればそれを保存する",
			utterances: evidenceTestUtterances(),
			responses: []string{
				reportJSON(t, genuineEvidence(), withKey(genuineTeacherEvidence(), "specificity", fabricated)),
				reportJSON(t, genuineEvidence(), genuineTeacherEvidence()),
			},
			wantCalls:    2,
			wantEvidence: genuineEvidence(),
			wantTeacher:  genuineTeacherEvidence(),
			why:          "教員向けの捏造では再生成が走っていない",
		},
		{
			name: "受験者の発話が無ければ全項目が空になるがレポートは保存する",
			utterances: []models.InterviewUtterance{
				{Role: "ai", Text: "学生時代に力を入れたことを教えてください。"},
			},
			responses: []string{reportJSON(t, genuineEvidence(), genuineTeacherEvidence())},
			wantCalls: reportGenerationAttempts,
			wantEvidence: map[string]string{
				"logic": "", "specificity": "", "ownership": "", "communication": "", "enthusiasm": "",
			},
			wantTeacher: map[string]string{"logic": "", "ownership": ""},
			why:         "照合先が無いのに根拠を保存している、またはレポートごと落としている",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			report, calls := runReport(t, tt.utterances, tt.responses...)
			if got := calls(); got != tt.wantCalls {
				t.Errorf("LLM呼び出し=%d want %d", got, tt.wantCalls)
			}

			gotEvidence := savedEvidence(t, report)
			for key, want := range tt.wantEvidence {
				if gotEvidence[key] != want {
					t.Errorf("evidence[%s]=%q want %q: %s", key, gotEvidence[key], want, tt.why)
				}
			}
			gotTeacher := savedTeacherEvidence(t, report)
			for key, want := range tt.wantTeacher {
				if gotTeacher[key] != want {
					t.Errorf("teacher.detailed_evidence[%s]=%q want %q: %s", key, gotTeacher[key], want, tt.why)
				}
			}
			// 根拠を落としてもスコアと講評は残す（誤りを載せるより欠落させる）
			if report.ScoresJSON == "" {
				t.Error("根拠の照合に失敗したせいでスコアまで捨てている")
			}
			if report.SummaryText == "" {
				t.Error("講評まで捨てている")
			}
		})
	}
}

// 捏造された根拠が保存内容のどこにも残らないこと（#1527）。
//
// 上のテーブルはキー単位で空かどうかを見るが、教員向けは JSON 全体が
// 1カラムに入るため、別のキーへ混ざっても気付けない。文字列として消えたことを確認する。
func TestGenerateReport_FabricatedTextIsGone(t *testing.T) {
	t.Parallel()

	report, _ := runReport(t, evidenceTestUtterances(),
		reportJSON(t,
			map[string]string{"logic": fabricated},
			map[string]string{"logic": "「" + fabricated + "」と述べており、語学力の裏付けがある。"}))

	for _, saved := range []struct{ name, body string }{
		{"evidence_json", report.EvidenceJSON},
		{"teacher_report_json", report.TeacherReportJSON},
	} {
		if strings.Contains(saved.body, "TOEIC") {
			t.Errorf("%s に捏造された根拠が残っている: %s", saved.name, saved.body)
		}
	}
}

// TestGenerateReport_KeepsValidCandidateWhenRetryBreaks は #1527 レビューで
// 見つかった回帰の固定。
//
// 照合のために再生成するようになったため、「attempt 1 は妥当だが attempt 2 が壊れる」
// という経路が新たに生まれた。最後の候補だけを見ていると、壊れた attempt 2 のせいで
// attempt 1 の妥当なスコアと照合済みの根拠まで捨てられる。
// これは #795（スコアが不正ならスコアだけ捨てる）とは別の話で、
// 「根拠の照合に失敗してもスコアは捨てない」という方針にも反する。
func TestGenerateReport_KeepsValidCandidateWhenRetryBreaks(t *testing.T) {
	t.Parallel()

	const notJSON = "申し訳ありませんが、レポートを生成できませんでした。"
	brokenScores := `{"summary":"s","scores":{"logic":99,"specificity":2,"ownership":4,"communication":3,"enthusiasm":4},` +
		`"evidence":{"logic":"根拠"},"strengths":["a"],"improvements":["b"]}`

	// attempt 1: スコア妥当・specificity だけ未照合（照合率 4/5 で再生成が走る）
	attempt1 := reportJSON(t,
		map[string]string{
			"logic":         genuineEvidence()["logic"],
			"specificity":   fabricated,
			"ownership":     genuineEvidence()["ownership"],
			"communication": genuineEvidence()["communication"],
			"enthusiasm":    genuineEvidence()["enthusiasm"],
		},
		genuineTeacherEvidence())

	tests := []struct {
		name      string
		attempt2  string
		wantScore bool
		why       string
	}{
		{
			name:      "2回目がJSONとして壊れていても1回目のスコアと根拠を残す",
			attempt2:  notJSON,
			wantScore: true,
			why:       "再生成が壊れたせいで妥当なスコアまで捨てている",
		},
		{
			name:      "2回目のスコアが値域外でも1回目のスコアと根拠を残す",
			attempt2:  brokenScores,
			wantScore: true,
			why:       "再生成のスコア違反に巻き込まれて1回目の妥当なスコアを捨てている",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			report, calls := runReport(t, evidenceTestUtterances(), attempt1, tt.attempt2)
			if got := calls(); got != 2 {
				t.Fatalf("LLM呼び出し=%d want 2", got)
			}
			if report.ScoresJSON == "" {
				t.Fatalf("スコアが捨てられている: %s", tt.why)
			}
			var scores map[string]int
			if err := json.Unmarshal([]byte(report.ScoresJSON), &scores); err != nil {
				t.Fatalf("scores_json が読めない: %v", err)
			}
			if scores["logic"] != 3 {
				t.Errorf("1回目の妥当なスコアが残っていない: %v", scores)
			}

			// 照合できた根拠は残り、未照合の1件だけが空になる
			got := savedEvidence(t, report)
			if got["logic"] != genuineEvidence()["logic"] {
				t.Errorf("照合できた根拠まで捨てている: %q", got["logic"])
			}
			if got["specificity"] != "" {
				t.Errorf("未照合の根拠が残っている: %q", got["specificity"])
			}
		})
	}
}

// 全試行のスコアが不正なら、#795 どおり evidence も含めてまとめて捨てる。
func TestGenerateReport_AllAttemptsInvalidScores(t *testing.T) {
	t.Parallel()

	broken := `{"summary":"落ち着いて回答できていました。","scores":{"logic":99,"specificity":2,"ownership":4,"communication":3,"enthusiasm":4},` +
		`"evidence":{"logic":"結果として、翌年の新入部員は23人まで増えて、部員数を3倍にすることができました。"},` +
		`"strengths":["a"],"improvements":["b"]}`

	report, _ := runReport(t, evidenceTestUtterances(), broken)
	if report.ScoresJSON != "" || report.EvidenceJSON != "" {
		t.Errorf("不正なスコアが残っている: scores=%q evidence=%q", report.ScoresJSON, report.EvidenceJSON)
	}
	if report.SummaryText == "" {
		t.Error("講評まで捨てている")
	}
}
