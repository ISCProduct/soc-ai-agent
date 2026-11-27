package repositories

import "testing"

func TestParseScoreSnapshot_旧形式と版付きのcategoriesを読む(t *testing.T) {
	legacy, ok := parseScoreSnapshot(`{"技術志向":80}`)
	if !ok || legacy["技術志向"] != 80 {
		t.Fatalf("legacy = %#v ok=%v", legacy, ok)
	}
	versioned, ok := parseScoreSnapshot(`{"version":2,"chat_weight":0.5,"categories":{"技術志向":60},"match_score":70,"axis_match":{"技術志向":55}}`)
	if !ok || versioned["技術志向"] != 60 {
		t.Fatalf("versioned = %#v ok=%v", versioned, ok)
	}
	if _, ok := versioned["match_score"]; ok {
		t.Fatal("総合マッチ度をカテゴリスコアとして読んでいる")
	}
	if _, ok := parseScoreSnapshot(`{"version":2,"chat_weight":0.5,"match_score":70}`); ok {
		t.Fatal("カテゴリが無い文書を較正入力にしている")
	}
}
