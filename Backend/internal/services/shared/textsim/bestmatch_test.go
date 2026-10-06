package textsim

import (
	"math"
	"testing"
)

// interviewSpoken は面接1回の発話を模した照合先。
// BestMatch の存在理由は「長い haystack でも短い引用を拾えること」なので、
// 短い文で試すと差が出ない。
func interviewSpoken() Bigrams {
	return New(`はい、私は大学時代に軽音サークルの代表を務めていました。
入学した当初は部員が8人しかいなくて、このままだと廃部になるという話が出ていました。
そこで私が新歓ライブの企画を提案して、SNSでの告知を担当しました。
結果として、翌年の新入部員は23人まで増えて、部員数を3倍にすることができました。
えー、その、あ、すみません、言い直します。プログラミングは独学で、毎日2時間くらい続けています。`)
}

// BestMatch の実測値を固定する（#1527）。
//
// 呼び出し側のしきい値（interview の EvidenceMatchThreshold）は、ここで測った値の
// 分布から決めている。アルゴリズムや Normalize を変えると分布ごと動くため、
// 代表値を固定して「静かにずれた」ことに気付けるようにする。
// 値が動いたら docs/wiki/scoring.md §2-4 の実測表も引き直すこと。
func TestBestMatch_PinnedScores(t *testing.T) {
	t.Parallel()

	spoken := interviewSpoken()
	tests := []struct {
		name   string
		needle string
		want   float64
	}{
		{"長い文の完全一致", "結果として、翌年の新入部員は23人まで増えて、部員数を3倍にすることができました。", 1},
		{"短い断片の完全一致", "私が新歓ライブの企画を提案して", 1},
		{"表記ゆれ（全角数字・句読点なし）", "翌年の新入部員は２３人まで増えて 部員数を３倍にすることができました", 1},
		{"助詞違い", "私が新歓ライブの企画も提案し、SNSでの告知も担当しました", 0.7778},
		{"言い直しを除いた引用", "プログラミングは独学で毎日2時間続けている", 0.8000},
		{"要約", "新歓ライブを企画してSNS告知を担当し、部員数を3倍にした", 0.5556},
		{"要約（最も低いもの）", "部員が8人から23人に増えた", 0.3200},
		// しきい値近傍。ここが動くと照合の合否が入れ替わる
		{"境界の上側", "自分から提案した", 0.2857},
		{"境界の下側", "顧問と相談した", 0.1818},
		{"捏造（述語も流用しない）", "TOEICで900点を取得し、英語での商談経験もあります。", 0.0392},
		// 以下は既知の限界（#1566）。高く出てしまうことを固定する
		{"捏造（述語末尾を流用）", "国際特許を3件取得することができました", 0.5556},
		{"実引用＋捏造の継ぎ足し", "入学した当初は部員が8人しかいなくて、私が部長として3年間で部員数を50人まで増やしました", 0.4500},
		{"フィラーをそのまま引用", "はい", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := BestMatch(tt.needle, spoken)
			if math.Abs(got-tt.want) > 0.0001 {
				t.Errorf("BestMatch = %.4f, want %.4f", got, tt.want)
			}
		})
	}
}

// 空・不正な入力で照合成功にしないこと。
// needle が空で 1.0 を返すと、記号だけの引用が「照合済み」として保存される。
func TestBestMatch_EmptyIsNotAMatch(t *testing.T) {
	t.Parallel()

	hay := New("部員数を3倍にしました")
	tests := []struct {
		name     string
		needle   string
		haystack Bigrams
	}{
		{"needle が空", "", hay},
		{"needle が記号だけ", "。。。！？", hay},
		{"needle が絵文字だけ", "🎉🎉🎉", hay},
		{"haystack が空", "部員数を3倍にした", New("")},
		{"haystack が記号だけ", "部員数を3倍にした", New("。。。")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := BestMatch(tt.needle, tt.haystack); got != 0 {
				t.Errorf("BestMatch = %v, want 0", got)
			}
		})
	}
}

// 長い haystack でも短い引用を拾えること。これが BestMatch の存在理由。
// MatchScore 単体だと Dice の分母が膨らんで要約が沈む。
func TestBestMatch_BeatsWholeTextScoreForSummaries(t *testing.T) {
	t.Parallel()

	spoken := interviewSpoken()
	const summary = "新歓ライブを企画してSNS告知を担当し、部員数を3倍にした"

	whole := spoken.MatchScore(New(summary))
	best := BestMatch(summary, spoken)
	if best <= whole {
		t.Errorf("BestMatch=%.4f が全体比較 MatchScore=%.4f を上回っていない", best, whole)
	}
	if whole >= 0.25 {
		t.Errorf("全体比較でも %.4f 出ており、窓をすべらせる意味が無い（前提が変わった）", whole)
	}
}

// needle が haystack より長いケース（要約されていない冗長な引用）でも落ちないこと。
func TestBestMatch_NeedleLongerThanHaystack(t *testing.T) {
	t.Parallel()
	hay := New("部員数を3倍にしました")
	if got := BestMatch("部員数を3倍にしました、と申し上げました通りです", hay); got < 0.5 {
		t.Errorf("BestMatch = %.3f, want >= 0.5", got)
	}
}
