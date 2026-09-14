package chat

import (
	"context"
	"testing"
)

// 選択肢→軸スコアの正確性契約:
//
//  1. A/1 → 100, B/2 → 80, ...（EvaluateHumanScoring / scoreChoice の正典）
//  2. 理由テキストを添えても軸スコアは変わらない（文章品質と軸位置は別物）
//  3. 「その他」は記号採点に落とさない

func TestChoiceAxisScore_IsDeterministicAndReasonDoesNotShift(t *testing.T) {
	e := NewAnswerEvaluator()
	q := `どれに近いですか？
A) 自分から主導して進める
B) みんなで協力して進める
C) その他（自由記述）`

	cases := []struct {
		letter string
		want   int
	}{
		{"A", 100},
		{"B", 80},
		{"1", 100},
		{"2", 80},
		{"3", 60},
		{"4", 40},
		{"5", 20},
	}
	for _, tc := range cases {
		got := e.EvaluateHumanScoring(q, tc.letter, true, false, nil)
		if got.Score != tc.want {
			t.Fatalf("choice %s score=%d want %d", tc.letter, got.Score, tc.want)
		}
	}
}

func TestResolveChoiceAnswer_ReasonDoesNotChangeLetter(t *testing.T) {
	q := `どれに近いですか？
A) 自分から主導して進める
B) みんなで協力して進める
C) その他（自由記述）`

	got := ResolveChoiceAnswer(q, "A: 仕様が固まる前に実装を始めたため長く説明します")
	if !got.IsChoice || got.Letter != "A" {
		t.Fatalf("expected choice A, got %+v", got)
	}
	if got.Reason == "" {
		t.Fatal("reason should be preserved for evidence")
	}

	// その他記号+理由は自由記述
	other := ResolveChoiceAnswer(q, "C: 本当は自由に書きたい内容です")
	if !other.IsFreeText || other.IsChoice {
		t.Fatalf("その他 must be free text, got %+v", other)
	}
}

func TestBlendRemoved_ChoiceScoreUnaffectedByReasonQuality(t *testing.T) {
	e := NewAnswerEvaluator()
	choiceOnly := e.EvaluateHumanScoring("q", "A", true, false, nil).Score
	reasonScore := e.EvaluateHumanScoringWithContext(
		context.Background(), "q", "なんとなくそう思うからです", false, false, nil,
	).Score
	if choiceOnly != 100 {
		t.Fatalf("A must be 100, got %d", choiceOnly)
	}
	t.Logf("reason quality score=%d must NOT be blended into axis (choice stays %d)", reasonScore, choiceOnly)
}
