package interview

import (
	"errors"
	"testing"
)

// 保存してよい内容の決め方(generateReportPayload の後半)を固定する。
// LLM を呼ばず、試行の結果だけを渡して確かめる。

func TestSettleReportPayload_BlanksUnmatchedEvidenceButKeepsScores(t *testing.T) {
	best := &reportCandidate{
		payload: reportPayload{
			Summary:  "講評",
			Scores:   map[string]int{"logic": 4},
			Evidence: map[string]string{"logic": "捏造された引用", "other": "実在の引用"},
		},
		evidence:  EvidenceCheck{Checked: 2, Unmatched: []string{"logic"}},
		unmatched: []string{"logic"},
	}

	got, err := settleReportPayload(1, best, reportPayload{}, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Evidence["logic"] != "" {
		t.Fatalf("照合できなかった根拠が残っている: %q", got.Evidence["logic"])
	}
	if got.Evidence["other"] != "実在の引用" {
		t.Fatalf("照合できた根拠まで消えている: %q", got.Evidence["other"])
	}
	if got.Summary != "講評" || got.Scores["logic"] != 4 {
		t.Fatalf("スコアと講評は残す: %+v", got)
	}
}

func TestSettleReportPayload_PrefersBestOverLaterBrokenAttempt(t *testing.T) {
	// 妥当な候補(best)があれば、後続の試行が壊れていて lastErr が立っていても、
	// スコアを捨てずに best を保存する。
	best := &reportCandidate{payload: reportPayload{Summary: "ok", Scores: map[string]int{"logic": 3}}}

	got, err := settleReportPayload(1, best, reportPayload{Summary: "壊れた2回目"}, true, errors.New("json broken"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != "ok" || got.Scores["logic"] != 3 {
		t.Fatalf("best を保存していない: %+v", got)
	}
}

func TestSettleReportPayload_NoBest(t *testing.T) {
	t.Run("本文は読めたがスコア不正なら講評だけ残す", func(t *testing.T) {
		p := reportPayload{Summary: "講評", Scores: map[string]int{"logic": 99}}
		got, err := settleReportPayload(1, nil, p, true, errors.New("score invalid"))
		if err != nil {
			t.Fatal(err)
		}
		if got.Summary != "講評" || len(got.Scores) != 0 {
			t.Fatalf("講評は残してスコアは捨てる: %+v", got)
		}
	})
	t.Run("本文も読めなければエラー", func(t *testing.T) {
		if _, err := settleReportPayload(1, nil, reportPayload{}, false, errors.New("json broken")); err == nil {
			t.Fatal("何も残せないときはエラーを返す")
		}
	})
}

// スコアを捨てた場合は ScoresJSON を空文字にする。"null" だと
// UpdateScoresFromInterviewReport の早期リターンに乗らず、スコア反映へ進んでしまう。
func TestBuildReportRecord_DroppedScoresAreEmptyString(t *testing.T) {
	s := &InterviewService{}

	dropped := s.buildReportRecord(5, reportPayload{Summary: "講評"})
	if dropped.ScoresJSON != "" {
		t.Fatalf("ScoresJSON = %q, want 空文字", dropped.ScoresJSON)
	}
	if dropped.SessionID != 5 || dropped.SummaryText != "講評" || dropped.TeacherReportJSON != "{}" {
		t.Fatalf("record = %+v", dropped)
	}

	kept := s.buildReportRecord(5, reportPayload{Scores: map[string]int{"logic": 4}})
	if kept.ScoresJSON == "" {
		t.Fatal("スコアがあるのに ScoresJSON が空")
	}
}
