package chat

import (
	"testing"
)

// 選択肢→軸スコアの品質契約:
//
//  1. scoreChoice の正典は A=100…E=20（生の位置）
//  2. 理由なしは中立寄りに減衰（極端な未根拠スコアを書かない）
//  3. 支持する理由があれば本値を採用
//  4. 矛盾する理由は強く減衰
//  5. 「その他」は記号採点に落とさない

func TestChoiceAxisScore_RawMapping(t *testing.T) {
	e := NewAnswerEvaluator()
	cases := []struct {
		letter string
		want   int
	}{
		{"A", 100}, {"B", 80}, {"C", 60}, {"D", 40}, {"E", 20},
		{"1", 100}, {"2", 80},
	}
	for _, tc := range cases {
		got := e.EvaluateHumanScoring("q", tc.letter, true, false, nil)
		if got.Score != tc.want {
			t.Fatalf("choice %s raw score=%d want %d", tc.letter, got.Score, tc.want)
		}
	}
}

func TestChoiceAxisScore_QualityAdjustmentPipeline(t *testing.T) {
	e := NewAnswerEvaluator()
	raw := e.EvaluateHumanScoring("q", "A", true, false, nil).Score

	only, _ := AdjustChoiceAxisScore(raw, "")
	if only >= raw {
		t.Fatalf("choice-only must dampen extreme %d -> %d", raw, only)
	}

	full, _ := AdjustChoiceAxisScore(raw, "チームで進めるのが好きで調整役をしてきました")
	if full != raw {
		t.Fatalf("supporting reason must keep raw %d, got %d", raw, full)
	}

	contra, _ := AdjustChoiceAxisScore(raw, "チームワークは苦手で一人で進めたいです")
	if contra >= only {
		t.Fatalf("contradiction (%d) should be <= choice-only (%d)", contra, only)
	}
}

func TestResolveChoiceAnswer_OtherStaysFreeText(t *testing.T) {
	q := `どれに近いですか？
A) 自分から主導して進める
B) みんなで協力して進める
C) その他（自由記述）`

	other := ResolveChoiceAnswer(q, "C: 本当は自由に書きたい内容です")
	if !other.IsFreeText || other.IsChoice {
		t.Fatalf("その他 must be free text, got %+v", other)
	}
	if other.Text != "本当は自由に書きたい内容です" {
		t.Fatalf("reason text=%q", other.Text)
	}
}
