package sttbench

import (
	"math"
	"testing"
)

// 表記ゆれを誤変換として数えると、実際には問題ない差でモデルを不当に低く評価する。
// 実測で「30パーセント → 30%」だけで CER 0.08 が出た。
func TestSemanticCER_IgnoresOrthography(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		hyp  string
	}{
		{"パーセント表記", "件数を30パーセント減らしました", "件数を30%減らしました"},
		{"ウェブ表記", "Webアプリケーションを開発しました", "ウェブアプリケーションを開発しました"},
		{"読点の有無", "私の強みは、やり切る力です", "私の強みはやり切る力です"},
		{"完全一致", "GoとAWSを使いました", "GoとAWSを使いました"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SemanticCER(tt.ref, tt.hyp); got != 0 {
				t.Errorf("SemanticCER = %v, want 0（表記差は意味の誤りではない）", got)
			}
		})
	}
}

// 意味が変わる誤りは必ず拾う。ここを緩めると評価の意味が無くなる。
func TestSemanticCER_CatchesMeaningChange(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		hyp  string
	}{
		{"数値が違う", "作業時間を年間120時間削減しました", "作業時間を年間20時間削減しました"},
		{"固有名詞が違う", "MySQLを採用しています", "MyScriptを採用しています"},
		{"役割が変わる", "私が主体となって進めました", "先輩が主体となって進めました"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SemanticCER(tt.ref, tt.hyp); got == 0 {
				t.Errorf("SemanticCER = 0, want > 0（意味が変わっている）")
			}
		})
	}
}

func TestCER(t *testing.T) {
	if got := CER("", ""); got != 0 {
		t.Errorf("空同士 = %v, want 0", got)
	}
	if got := CER("", "何か"); got != 1 {
		t.Errorf("正解が空で認識あり = %v, want 1", got)
	}
	if got := CER("あいう", "あいえ"); math.Abs(got-1.0/3.0) > 1e-9 {
		t.Errorf("1文字違い = %v, want 1/3", got)
	}
}

// 面接では「120時間」を「20時間」と誤ると成果の意味が変わる。
// 一般的なCERでは1文字差にしかならず埋もれるため別指標にしている。
func TestNumberAccuracy(t *testing.T) {
	tests := []struct {
		name              string
		ref, hyp          string
		wantMatch, wantTo int
	}{
		{"全一致", "120時間と30パーセント", "120時間と30%", 2, 2},
		{"1つ誤り", "120時間と30パーセント", "20時間と30%", 1, 2},
		{"全角数字も同一視", "１２０時間", "120時間", 1, 1},
		{"数値なし", "頑張りました", "頑張りました", 0, 0},
		{"認識側に余分があっても正解数は変わらない", "30パーセント", "30%と40%", 1, 1},
		{"同じ数値が2回", "3回と3人", "3回と3人", 2, 2},
		{"同じ数値が1つ落ちる", "3回と3人", "3回と人", 1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, total := NumberAccuracy(tt.ref, tt.hyp)
			if m != tt.wantMatch || total != tt.wantTo {
				t.Errorf("= (%d, %d), want (%d, %d)", m, total, tt.wantMatch, tt.wantTo)
			}
		})
	}
}

// checks には実際に現れる語と評価観点のラベルが混ざっている。
// ラベルまで照合すると固有名詞の正解率を実態より大幅に低く見せる（実測で60%）。
func TestCheckableKeywords(t *testing.T) {
	ref := "GoとAWSを使って、Docker上で動くバックエンドAPIを実装しました。"
	got := CheckableKeywords([]string{"Go", "AWS", "Docker", "一般語", "自然な文章"}, ref)
	want := []string{"Go", "AWS", "Docker"}
	if len(got) != len(want) {
		t.Fatalf("= %v, want %v（評価ラベルは除外されるべき）", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestKeywordHits_CaseInsensitive(t *testing.T) {
	hit, miss := KeywordHits([]string{"AWS", "MySQL"}, "awsとmysqlを使いました")
	if len(hit) != 2 || len(miss) != 0 {
		t.Errorf("hit=%v miss=%v（大文字小文字は同義）", hit, miss)
	}
}

// 空文字だけでなく、音声長に対して極端に短い結果も失敗として扱う。
// 数文字だけ返ると、後段のLLMは無言と区別できず会話が破綻する。
func TestIsRecognitionFailure(t *testing.T) {
	tests := []struct {
		name string
		text string
		sec  float64
		want bool
	}{
		{"空文字", "", 10, true},
		{"空白のみ", "   ", 10, true},
		{"10秒で3文字は失敗", "はい。", 10, true},
		{"10秒で十分な長さ", "本日はお時間をいただきありがとうございます。よろしくお願いします。", 10, false},
		{"短い発話の短い結果は失敗ではない", "はい。", 1, false},
		{"長さ不明なら長さ判定をしない", "はい。", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRecognitionFailure(tt.text, tt.sec, 1.0); got != tt.want {
				t.Errorf("= %v, want %v", got, tt.want)
			}
		})
	}
}
