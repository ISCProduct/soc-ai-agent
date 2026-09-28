package textsim

import (
	"math"
	"testing"
)

func TestNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"全角英数を半角に畳む", "ＡＢＣ１２３", "abc123"},
		{"半角カナを全角に畳む", "ｻｰｸﾙ", "サークル"},
		{"句読点と空白を落とす", "部員数を、3倍に しました。", "部員数を3倍にしました"},
		{"括弧や引用符も落とす", "「代表」を務めた（2年間）", "代表を務めた2年間"},
		{"記号・絵文字を落とす", "3倍にした🎉👏", "3倍にした"},
		{"改行を落とす", "一行目\n二行目", "一行目二行目"},
		{"数字に挟まれた小数点は残す", "1.5倍に伸びた", "1.5倍に伸びた"},
		{"桁区切りのカンマは落とす", "1,200人が参加", "1200人が参加"},
		{"数字に挟まれていない句点は落とす", "増えました。次は", "増えました次は"},
		{"空文字", "", ""},
		{"記号だけなら空になる", "。、！？ ", ""},
		{"絵文字だけなら空になる", "🎉🎉🎉", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Normalize(tt.in); got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// 「1.5倍」と「15倍」が同一視されないこと。
// 正規化で小数点を落とすと、数値の捏造が引用として通ってしまう。
func TestNormalize_KeepsDecimalPoint(t *testing.T) {
	t.Parallel()
	if Normalize("1.5倍") == Normalize("15倍") {
		t.Error("1.5倍 と 15倍 が同一視されている")
	}
}

// 桁区切りのカンマは書式差にすぎないので、有無で一致度が落ちないこと。
// 元テキストが「1,200」でLLMが「1200」と書いた（または逆の）場合に落ちるのは損。
func TestNormalize_IgnoresThousandsSeparator(t *testing.T) {
	t.Parallel()
	if Normalize("売上1,200万円") != Normalize("売上1200万円") {
		t.Errorf("桁区切りの有無で別テキストになっている: %q / %q",
			Normalize("売上1,200万円"), Normalize("売上1200万円"))
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        string
		wantLen   int
		wantEmpty bool
	}{
		{"通常", "部員数を、3倍に", 7, false},
		{"空文字", "", 0, true},
		{"記号だけ", "。。。！？", 0, true},
		{"1文字", "私", 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := New(tt.in)
			if got.Len() != tt.wantLen {
				t.Errorf("Len() = %d, want %d", got.Len(), tt.wantLen)
			}
			if got.Empty() != tt.wantEmpty {
				t.Errorf("Empty() = %v, want %v", got.Empty(), tt.wantEmpty)
			}
		})
	}
}

// MatchScore は非対称。レシーバが探される側。
func TestMatchScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		haystack         string
		needle           string
		want             float64
		wantMin, wantMax float64
	}{
		{name: "完全一致", haystack: "サークルの代表", needle: "サークルの代表", want: 1},
		{name: "needle が haystack の部分文字列なら1.0", haystack: "私はサークルの代表を務めていました", needle: "サークルの代表", want: 1},
		{name: "表記ゆれだけの部分文字列も1.0", haystack: "部員数を3倍にしました", needle: "部員数を３倍に、", want: 1},
		{name: "両方空なら0（照合できたとはみなさない）", haystack: "", needle: "", want: 0},
		{name: "haystack だけ空", haystack: "", needle: "代表", want: 0},
		{name: "needle だけ空", haystack: "代表", needle: "", want: 0},
		{name: "記号だけの needle は空扱い", haystack: "代表を務めた", needle: "🎉🎉🎉", want: 0},
		{name: "共通部分なし", haystack: "野球部", needle: "簿記検定", want: 0},
		{name: "1文字同士の一致", haystack: "あ", needle: "あ", want: 1},
		{name: "1文字同士の不一致", haystack: "あ", needle: "い", want: 0},
		{name: "部分的に重なる", haystack: "部員数を3倍にした", needle: "部員数を2倍にした", wantMin: 0.5, wantMax: 0.99},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := New(tt.haystack).MatchScore(New(tt.needle))
			if tt.wantMax == 0 {
				if got != tt.want {
					t.Errorf("MatchScore(%q <- %q) = %v, want %v", tt.haystack, tt.needle, got, tt.want)
				}
				return
			}
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("MatchScore(%q <- %q) = %v, want %v〜%v", tt.haystack, tt.needle, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// 包含は片側だけ。長い文を短い文で探しても1.0にはならない。
// 対称にすると「発話の一部を引用した」と「発話に無い内容を足した」が区別できない。
func TestMatchScore_Asymmetric(t *testing.T) {
	t.Parallel()
	long, short := New("私はサークルの代表を務めていました"), New("サークルの代表")
	if got := long.MatchScore(short); got != 1 {
		t.Errorf("長.MatchScore(短) = %v, want 1", got)
	}
	if got := short.MatchScore(long); got == 1 {
		t.Error("短.MatchScore(長) が 1 になっている。包含判定が双方向になっている")
	}
}

// BestMatch の実測値を固定する（#1527）。
//
// しきい値（呼び出し側の EvidenceMatchThreshold）は、ここで測った値の分布から
// 決めている。アルゴリズムを変えると分布ごと動くため、代表値を固定して
// 「静かにずれた」ことに気付けるようにする。値を動かしたときは
// docs/wiki/scoring.md §2-4 の実測表も引き直すこと。
func TestBestMatch_PinnedScores(t *testing.T) {
	t.Parallel()

	haystack := New(`はい、私は大学時代に軽音サークルの代表を務めていました。
入学した当初は部員が8人しかいなくて、このままだと廃部になるという話が出ていました。
そこで私が新歓ライブの企画を提案して、SNSでの告知を担当しました。
結果として、翌年の新入部員は23人まで増えて、部員数を3倍にすることができました。
えー、その、あ、すみません、言い直します。プログラミングは独学で、毎日2時間くらい続けています。`)

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
		{"捏造（別エピソード）", "TOEICで900点を取得し、英語での商談経験もあります。", 0.0392},
		// 既知の限界: 実引用＋捏造の継ぎ足しは正当な要約と同じ帯に入る
		{"実引用＋捏造の継ぎ足し", "入学した当初は部員が8人しかいなくて、私が部長として3年間で部員数を50人まで増やしました", 0.4500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := BestMatch(tt.needle, haystack)
			if math.Abs(got-tt.want) > 0.0001 {
				t.Errorf("BestMatch = %.4f, want %.4f", got, tt.want)
			}
		})
	}
}

// 空・不正な入力で照合成功にしないこと。
// needle が空で1.0を返すと、記号だけの引用が「照合済み」として保存される。
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

// needle が haystack より長いケース（要約されていない冗長な引用）でも落ちないこと。
func TestBestMatch_NeedleLongerThanHaystack(t *testing.T) {
	t.Parallel()
	hay := New("部員数を3倍にしました")
	if got := BestMatch("部員数を3倍にしました、と申し上げました通りです", hay); got < 0.5 {
		t.Errorf("BestMatch = %.3f, want >= 0.5", got)
	}
}
