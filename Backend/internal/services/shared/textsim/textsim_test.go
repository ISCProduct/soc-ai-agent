package textsim

import "testing"

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
		{"括弧や記号も落とす", "「代表」を務めた（2年間）", "代表を務めた2年間"},
		{"改行を落とす", "一行目\n二行目", "一行目二行目"},
		{"空文字", "", ""},
		{"記号だけなら空になる", "。、！？ ", ""},
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

func TestSimilarity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		a, b    string
		want    float64
		wantMin float64
		wantMax float64
	}{
		{name: "完全一致", a: "サークルの代表", b: "サークルの代表", want: 1},
		{name: "両方空", a: "", b: "", want: 1},
		{name: "片方だけ空", a: "代表", b: "", want: 0},
		{name: "共通部分なし", a: "野球部", b: "簿記検定", want: 0},
		{name: "1文字同士の一致", a: "あ", b: "あ", want: 1},
		{name: "1文字同士の不一致", a: "あ", b: "い", want: 0},
		{name: "部分的に重なる", a: "部員数を3倍にした", b: "部員数を2倍にした", wantMin: 0.5, wantMax: 0.99},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Similarity(tt.a, tt.b)
			if tt.wantMax == 0 {
				if got != tt.want {
					t.Errorf("Similarity(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
				}
				return
			}
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("Similarity(%q, %q) = %v, want %v〜%v", tt.a, tt.b, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// 事前計算した Text/Score と簡易版 Similarity が一致すること。
// ずれると「繰り返し比較する側だけ事前計算する」最適化が結果を変えてしまう。
func TestScore_MatchesSimilarity(t *testing.T) {
	t.Parallel()
	a, b := "新歓ライブを企画した", "新歓ライブの企画を提案した"
	ta := Text(a)
	if got, want := Score(ta, Text(b)), Similarity(a, b); got != want {
		t.Errorf("Score = %v, Similarity = %v", got, want)
	}
	if ta.Len() != len([]rune(a)) {
		t.Errorf("Len = %d, want %d", ta.Len(), len([]rune(a)))
	}
}

// Dice は対称であること。非対称だと引数の順番で結果が変わり、呼び出し側が壊れる。
func TestSimilarity_Symmetric(t *testing.T) {
	t.Parallel()
	a, b := "新歓ライブを企画した", "新歓ライブの企画を提案した"
	if Similarity(a, b) != Similarity(b, a) {
		t.Errorf("非対称: %v != %v", Similarity(a, b), Similarity(b, a))
	}
}

// BestMatch の肝は「長い haystack に短い needle が入っていても落ちない」こと。
// haystack 全体と素で Dice を取ると分母が膨らんで完全一致の引用まで落ちる。
func TestBestMatch(t *testing.T) {
	t.Parallel()

	haystack := Normalize(`はい、私は大学時代に軽音サークルの代表を務めていました。
入学した当初は部員が8人しかいなくて、このままだと廃部になるという話が出ていました。
そこで私が新歓ライブの企画を提案して、SNSでの告知を担当しました。
結果として、翌年の新入部員は23人まで増えて、部員数を3倍にすることができました。`)

	tests := []struct {
		name    string
		needle  string
		wantMin float64
		wantMax float64
	}{
		{"長い文の完全一致", "結果として、翌年の新入部員は23人まで増えて、部員数を3倍にすることができました。", 0.99, 1},
		{"短い断片の完全一致", "私が新歓ライブの企画を提案して", 0.99, 1},
		{"要約", "新歓ライブを企画してSNS告知を担当し、部員数を3倍にした", 0.4, 1},
		{"表記ゆれ（全角数字・句読点なし）", "翌年の新入部員は２３人まで増えて 部員数を３倍にすることができました", 0.99, 1},
		{"捏造", "TOEICで900点を取得し、英語での商談経験もあります。", 0, 0.2},
		{"needle が空なら照合対象が無い", "", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := BestMatch(Normalize(tt.needle), haystack)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("BestMatch = %.3f, want %v〜%v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestBestMatch_EmptyHaystack(t *testing.T) {
	t.Parallel()
	if got := BestMatch(Normalize("部員数を3倍にした"), ""); got != 0 {
		t.Errorf("BestMatch = %v, want 0（照合先が無ければ一致とみなさない）", got)
	}
}

// needle が haystack より長いケース（要約されていない冗長な引用）でも落ちないこと。
func TestBestMatch_NeedleLongerThanHaystack(t *testing.T) {
	t.Parallel()
	hay := Normalize("部員数を3倍にしました")
	got := BestMatch(Normalize("部員数を3倍にしました、と申し上げました通りです"), hay)
	if got < 0.5 {
		t.Errorf("BestMatch = %.3f, want >= 0.5", got)
	}
}
