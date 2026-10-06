package aibench

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"Backend/internal/services/interview"
	"Backend/internal/services/shared/textsim"
)

// 捏造判定（evidence_not_spoken）がゴールデンセットに対して正しく働くことを固定する。
//
// この判定はハーネスの数字を大きく動かす。実測で #1580（照合の内容語化）の前後で
// 検出数が 9 → 17 に増えたが、これが「検出が厳しくなった」のか「正当な要約まで
// 落ちるようになった」のかは、しきい値と実際の文面を突き合わせないと分からない。
// **この区別を間違えると「モデルが悪化した」という誤った結論になる。**
//
// 下の実測値（iv-g01 の発話に対する内容語 Dice 係数）が区別の根拠:
//
//	完全引用          1.000  照合
//	短い完全引用      1.000  照合
//	正当な要約        0.667  照合
//	---------------- しきい値 0.25 ----------------
//	面接官の発言      0.154  未照合（#1580 の狙い）
//	抽象的な講評      0.085  未照合
//	完全な捏造        0.000  未照合
//
// 正当な引用・要約は余裕を持って通るので、検出数の増加は誤検知ではなく
// 「モデルが引用ではなく抽象的な講評を書いている」ことを意味する。
func TestゴールデンセットでEvidence照合が正しく分かれる(t *testing.T) {
	path := "../../../docs/research/ai-eval/interview-report.jsonl"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s の読み込みに失敗: %v", path, err)
	}
	var target Case
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.Contains(line, `"iv-g01"`) {
			continue
		}
		if err := json.Unmarshal([]byte(line), &target); err != nil {
			t.Fatalf("iv-g01 の解析に失敗: %v", err)
		}
	}
	if target.ID == "" {
		t.Fatal("iv-g01 がゴールデンセットに無い（この検証の前提が崩れている）")
	}
	spoken := interview.SpokenContent(utterancesFromTranscript(target.Input.Transcript))

	tests := []struct {
		name      string
		text      string
		wantMatch bool
	}{
		{name: "発話の完全引用は照合できる", wantMatch: true,
			text: "毎日3,000円分のパンを捨てていたので、2か月間、時間帯別の売れ残りを記録して、17時以降に残る5種類を特定しました"},
		{name: "短い完全引用も照合できる", wantMatch: true,
			text: "廃棄額は1日900円まで下がりました"},
		{name: "語彙を引き継いだ要約は照合できる", wantMatch: true,
			text: "記録をもとに値引き運用と発注の見直しを店長へ提案し、廃棄額を1日900円まで削減しました"},
		// ここから下が未照合になるべきもの
		{name: "抽象的な講評は引用ではないので未照合", wantMatch: false,
			text: "課題を数値で把握したうえで具体的な改善策を提案し、成果を出しています"},
		{name: "一般的な講評は未照合", wantMatch: false,
			text: "論理的に筋道立てて説明できていました"},
		{name: "完全な捏造は未照合", wantMatch: false,
			text: "部活で全国大会に出場した経験を語っていました"},
		// #1527 の狙い: 面接官の質問文を根拠にしても通らない
		{name: "面接官の発言を根拠にしても未照合", wantMatch: false,
			text: "本日はよろしくお願いします。まず自己紹介と、学生時代に力を入れたことを教えてください。"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := textsim.ContentBestMatch(tt.text, spoken)
			matched := score >= interview.EvidenceMatchThreshold
			if matched != tt.wantMatch {
				t.Errorf("一致度 %.3f（しきい値 %.3f）→ 照合=%v, want %v",
					score, interview.EvidenceMatchThreshold, matched, tt.wantMatch)
			}
		})
	}
}
