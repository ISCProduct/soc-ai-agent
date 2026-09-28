package interview

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// reportJSON はレポート1件分の LLM 応答を組み立てる。
func reportJSON(t *testing.T, evidence map[string]string) string {
	t.Helper()
	body, err := json.Marshal(reportPayload{
		Summary:      "落ち着いて回答できていました。",
		Scores:       map[string]int{"logic": 3, "specificity": 2, "ownership": 4, "communication": 3, "enthusiasm": 4},
		Evidence:     evidence,
		Strengths:    []string{"結論から話せる"},
		Improvements: []string{"数値を添える"},
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

// TestGenerateReport_EvidenceVerification は #1527 の中心的な回帰テスト。
//
// evidence は自由記述なのでスキーマ検証では捏造を検出できない。
// 学生が言っていない発言が「根拠」として学生向け・教員向けの両レポートに出ると、
// レポートを前提に指導する教員の信頼を直接損なう。
// 実発話と照合して、照合できない項目は再生成 → それでも駄目なら空にして保存する。
func TestGenerateReport_EvidenceVerification(t *testing.T) {
	t.Parallel()

	genuine := map[string]string{
		"logic":         "結果として、翌年の新入部員は23人まで増えて、部員数を3倍にすることができました。",
		"specificity":   "入学した当初は部員が8人しかいなくて",
		"ownership":     "そこで私が新歓ライブの企画を提案して、SNSでの告知を担当しました。",
		"communication": "新歓ライブを企画してSNS告知を担当し、部員数を3倍にした",
		"enthusiasm":    "私は大学時代に軽音サークルの代表を務めていました",
	}
	// specificity だけ捏造。他は実発話に基づく。
	partlyFake := map[string]string{
		"logic":         genuine["logic"],
		"specificity":   "TOEICで900点を取得し、英語での商談経験もあります。",
		"ownership":     genuine["ownership"],
		"communication": genuine["communication"],
		"enthusiasm":    genuine["enthusiasm"],
	}

	tests := []struct {
		name         string
		utterances   []models.InterviewUtterance
		responses    []string
		wantCalls    int
		wantEvidence map[string]string
		why          string
	}{
		{
			name:         "全項目が実発話に基づくなら1回で確定する",
			utterances:   evidenceTestUtterances(),
			responses:    []string{reportJSON(t, genuine)},
			wantCalls:    1,
			wantEvidence: genuine,
			why:          "照合できているのに再生成している（LLM費用が二重に掛かる）",
		},
		{
			name:         "捏造があれば作り直し、2回目が通ればそれを保存する",
			utterances:   evidenceTestUtterances(),
			responses:    []string{reportJSON(t, partlyFake), reportJSON(t, genuine)},
			wantCalls:    2,
			wantEvidence: genuine,
			why:          "捏造された根拠をそのまま保存している",
		},
		{
			name:       "作り直しても捏造なら、その項目だけ空にして保存する",
			utterances: evidenceTestUtterances(),
			responses:  []string{reportJSON(t, partlyFake)},
			wantCalls:  reportGenerationAttempts,
			wantEvidence: map[string]string{
				"logic":         genuine["logic"],
				"specificity":   "", // 照合できなかった項目だけ欠落させる
				"ownership":     genuine["ownership"],
				"communication": genuine["communication"],
				"enthusiasm":    genuine["enthusiasm"],
			},
			why: "捏造された根拠が残っている、または照合できた項目まで捨てている",
		},
		{
			name: "受験者の発話が無ければ全項目が空になるがレポートは保存する",
			utterances: []models.InterviewUtterance{
				{Role: "ai", Text: "学生時代に力を入れたことを教えてください。"},
			},
			responses: []string{reportJSON(t, genuine)},
			wantCalls: reportGenerationAttempts,
			wantEvidence: map[string]string{
				"logic": "", "specificity": "", "ownership": "", "communication": "", "enthusiasm": "",
			},
			why: "照合先が無いのに根拠を保存している、またはレポートごと落としている",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			const sessionID = uint(7)
			sessionRepo := newSessionRepoStub(&models.InterviewSession{ID: sessionID, UserID: 3, Status: "finished", Language: "ja"})
			reportRepo := &reportRepoStub{}
			client, calls := chatStub(t, tt.responses...)
			svc := NewInterviewService(sessionRepo, &utterRepoStub{utterances: tt.utterances}, reportRepo, nil, nil, client, nil)

			if err := svc.generateReport(context.Background(), sessionID); err != nil {
				t.Fatalf("レポート生成が落ちた: %v", err)
			}
			if got := calls(); got != tt.wantCalls {
				t.Errorf("LLM呼び出し=%d want %d", got, tt.wantCalls)
			}
			if reportRepo.upsertCalls != 1 {
				t.Fatalf("Upsert=%d want 1: レポートが保存されていない", reportRepo.upsertCalls)
			}

			var gotEvidence map[string]string
			if err := json.Unmarshal([]byte(reportRepo.last.EvidenceJSON), &gotEvidence); err != nil {
				t.Fatalf("evidence_json が読めない (%q): %v", reportRepo.last.EvidenceJSON, err)
			}
			for key, want := range tt.wantEvidence {
				if gotEvidence[key] != want {
					t.Errorf("evidence[%s]=%q want %q: %s", key, gotEvidence[key], want, tt.why)
				}
			}
			// 根拠を落としてもスコアと講評は残す（誤りを載せるより欠落させる）
			if reportRepo.last.ScoresJSON == "" {
				t.Error("根拠の照合に失敗したせいでスコアまで捨てている")
			}
			if reportRepo.last.SummaryText == "" {
				t.Error("講評まで捨てている")
			}
		})
	}
}

// スコアが不正な候補で終わった場合は、evidence も含めてまとめて捨てる（#795 の挙動を壊さない）。
// 前の試行で照合した結果を持ち越して空文字を書き戻すと、キーだけ残って混乱する。
func TestGenerateReport_InvalidScoresAfterUnmatchedEvidence(t *testing.T) {
	t.Parallel()

	const sessionID = uint(8)
	fake := reportJSON(t, map[string]string{
		"logic":         "TOEICで900点を取得しました。",
		"specificity":   "TOEICで900点を取得しました。",
		"ownership":     "TOEICで900点を取得しました。",
		"communication": "TOEICで900点を取得しました。",
		"enthusiasm":    "TOEICで900点を取得しました。",
	})
	// 2回目はスコアが値域外
	broken := `{"summary":"s","scores":{"logic":99,"specificity":2,"ownership":4,"communication":3,"enthusiasm":4},` +
		`"evidence":{"logic":"根拠"},"strengths":["a"],"improvements":["b"]}`

	sessionRepo := newSessionRepoStub(&models.InterviewSession{ID: sessionID, UserID: 3, Status: "finished", Language: "ja"})
	reportRepo := &reportRepoStub{}
	client, _ := chatStub(t, fake, broken)
	svc := NewInterviewService(sessionRepo, &utterRepoStub{utterances: evidenceTestUtterances()}, reportRepo, nil, nil, client, nil)

	if err := svc.generateReport(context.Background(), sessionID); err != nil {
		t.Fatalf("レポート生成が落ちた: %v", err)
	}
	if reportRepo.last.ScoresJSON != "" || reportRepo.last.EvidenceJSON != "" {
		t.Errorf("不正なスコアが残っている: scores=%q evidence=%q",
			reportRepo.last.ScoresJSON, reportRepo.last.EvidenceJSON)
	}
	if reportRepo.last.SummaryText == "" {
		t.Error("講評まで捨てている")
	}
}
