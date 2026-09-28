// Package textsim は文字bigramによるテキスト類似度を提供する。
//
// 用途は「LLM が引用として返した文が、元テキストに実在するか」の照合。
// 形態素解析器も埋め込みAPIも使わない。文字bigramなら日本語でも語境界を
// 気にせず済み、ローカル計算だけで完結するため追加のLLM呼び出しが発生しない。
//
// 汎用パッケージなので、面接/履歴書などの固有概念は持ち込まないこと。
// しきい値も用途ごとに差があるため、ここには置かず呼び出し側の定数で持つ。
package textsim

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Bigrams は文字bigramを事前計算したテキスト。
// 同じテキストを繰り返し比較する側は Text で1度だけ作って使い回す。
type Bigrams struct {
	runes []rune
	set   map[string]struct{}
}

// Len は元テキストの文字数を返す。
func (b Bigrams) Len() int { return len(b.runes) }

// Normalize は照合前の正規化を行う。
//
// NFKC で全角/半角の揺れを畳み、空白と約物（句読点・記号）を落として小文字化する。
// 「〜しました。」と「〜しました」、「2倍」と「２倍」のような表記差で
// 照合が落ちるのを防ぐのが目的。助詞の違いはここでは吸収せず、しきい値側で許容する。
func Normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range norm.NFKC.String(s) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// Text は文字bigramを事前計算する。正規化はしない（必要なら Normalize を先に掛ける）。
func Text(s string) Bigrams {
	return fromRunes([]rune(s))
}

// fromRunes は1文字の入力ではその1文字自体を要素にする。
// bigram が作れないからといって空集合にすると、1文字同士の比較が必ず0になる。
func fromRunes(rs []rune) Bigrams {
	set := make(map[string]struct{}, len(rs))
	switch {
	case len(rs) == 0:
	case len(rs) == 1:
		set[string(rs)] = struct{}{}
	default:
		for i := range len(rs) - 1 {
			set[string(rs[i:i+2])] = struct{}{}
		}
	}
	return Bigrams{runes: rs, set: set}
}

// Score は事前計算済みテキスト同士の Dice 係数（0.0〜1.0）を返す。
// 両方が空なら1.0、片方だけ空なら0.0。
func Score(a, b Bigrams) float64 {
	if len(a.set) == 0 && len(b.set) == 0 {
		return 1
	}
	if len(a.set) == 0 || len(b.set) == 0 {
		return 0
	}
	small, large := a.set, b.set
	if len(large) < len(small) {
		small, large = large, small
	}
	inter := 0
	for g := range small {
		if _, ok := large[g]; ok {
			inter++
		}
	}
	return 2 * float64(inter) / float64(len(a.set)+len(b.set))
}

// Similarity は文字列を直接比較する簡易版。1回しか比べないときに使う。
func Similarity(a, b string) float64 {
	return Score(Text(a), Text(b))
}

// BestMatch は needle が haystack のどこかに現れる度合い（0.0〜1.0）を返す。
//
// haystack 全体と直接 Score を取ると、haystack が長いほど分母が膨らんで
// 完全一致の引用でも0に近づく。そこで needle と同じ長さの窓を1文字ずつ
// すべらせ、窓ごとの Dice の最大値を採る。要約された引用は元の発言より短いため、
// 2倍幅の窓も併せて見る（要約元のスパンが needle より長いケースを拾う）。
//
// 正規化はしない。呼び出し側が Normalize を掛けた文字列を渡すこと。
//
// ponytail: 窓を1文字ずつ総当たりする O(len(haystack)×len(needle)) の素朴な実装。
// 面接1回の発話（数千文字）× 引用5件なら数十msで済む。文書単位に広げるなら
// bigram の出現位置を先にインデックス化して候補窓を絞ること。
func BestMatch(needle, haystack string) float64 {
	n, h := Text(needle), Text(haystack)
	if n.Len() == 0 {
		return 1
	}
	if h.Len() == 0 {
		return 0
	}
	best := 0.0
	for _, w := range []int{n.Len(), n.Len() * 2} {
		if w >= h.Len() {
			// 窓が haystack 以上なら全体と1回比べるだけでよい
			best = max(best, Score(n, h))
			continue
		}
		for i := 0; i+w <= h.Len(); i++ {
			best = max(best, Score(n, fromRunes(h.runes[i:i+w])))
		}
	}
	return best
}
