// Package textsim は文字bigramによるテキスト類似度を提供する。
//
// 用途は「LLM が引用として返した文が、元テキストに実在するか」の照合。
// 形態素解析器も埋め込みAPIも使わない。文字bigramなら日本語でも語境界を
// 気にせず済み、ローカル計算だけで完結するため追加のLLM呼び出しが発生しない。
//
// 汎用パッケージなので、面接/履歴書などの固有概念は持ち込まないこと。
// しきい値も用途ごとに差があるため、ここには置かず呼び出し側の定数で持つ。
//
// このファイルは #1551（履歴書の引用照合）と同じ内容で、#1527 が追加したのは
// BestMatch だけである。#1551 がマージされたらこのファイルを削除し、
// BestMatch のみを #1551 版へ移植する。
//
// 照合が通っても「実発話に基づく」とは限らない。日本語の述語末尾を流用した
// 捏造はこの指標では弾けない（Issue #1566）。限界は呼び出し側で文書化すること。
package textsim

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Bigrams は正規化済みテキストと、その文字bigram集合。
// 同じテキストを繰り返し比較する側は New で1度だけ作って使い回す。
type Bigrams struct {
	norm    string
	runes   int
	bigrams map[string]struct{}
}

// New は正規化して文字bigramを事前計算する。Normalize を別途呼ぶ必要はない。
func New(s string) Bigrams {
	rs := []rune(Normalize(s))
	return Bigrams{norm: string(rs), runes: len(rs), bigrams: gramSet(rs)}
}

// Len は正規化後の文字数を返す。
func (t Bigrams) Len() int { return t.runes }

// Empty は正規化後に中身が残らなかったかを返す。
// 記号や絵文字だけの文字列はここで true になる。
func (t Bigrams) Empty() bool { return t.runes == 0 }

// Normalize は照合前の正規化を行う。
//
// NFKC で全角/半角の揺れを畳み、空白と約物（句読点・記号）を落として小文字化する。
// 「〜しました。」と「〜しました」、「2倍」と「２倍」のような表記差で
// 照合が落ちるのを防ぐのが目的。助詞の違いはここでは吸収せず、しきい値側で許容する。
//
// 数字に挟まれた "." は残す。落とすと「1.5倍」と「15倍」が同一視される。
// 一方で桁区切りの "," は落とす。「1,200」と「1200」は同じ数値の書式差にすぎず、
// 片方だけの表記で一致度が落ちるのは損でしかない。
func Normalize(s string) string {
	src := []rune(norm.NFKC.String(s))
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range src {
		if unicode.IsSpace(r) {
			continue
		}
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			if !(r == '.' && betweenDigits(src, i)) {
				continue
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func betweenDigits(src []rune, i int) bool {
	return i > 0 && i+1 < len(src) && unicode.IsDigit(src[i-1]) && unicode.IsDigit(src[i+1])
}

// gramSet は文字bigramの集合を返す。
// 1文字の入力ではその1文字自体を要素にする。bigram が作れないからといって
// 空集合にすると、1文字同士の比較が必ず0になる。
func gramSet(rs []rune) map[string]struct{} {
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
	return set
}

// MatchScore は needle がレシーバ（haystack）に現れる度合い（0.0〜1.0）を返す。
//
// 非対称である。レシーバが探される側、引数が探す側。
// needle が haystack の部分文字列なら 1.0（完全な引用）。
// そうでなければ bigram 集合の Dice 係数に委ねる。
// どちらかが空なら 0.0（照合できたとはみなさない）。
func (t Bigrams) MatchScore(needle Bigrams) float64 {
	if t.Empty() || needle.Empty() {
		return 0
	}
	if strings.Contains(t.norm, needle.norm) {
		return 1
	}
	return dice(t.bigrams, needle.bigrams)
}

func dice(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	small, large := a, b
	if len(large) < len(small) {
		small, large = large, small
	}
	inter := 0
	for g := range small {
		if _, ok := large[g]; ok {
			inter++
		}
	}
	return 2 * float64(inter) / float64(len(a)+len(b))
}

// BestMatch は needle が h のどこかに現れる度合い（0.0〜1.0）を返す（#1527 の追加分）。
//
// h 全体と MatchScore を取るだけでは、完全な引用（部分文字列）は 1.0 で拾えるが、
// 要約や言い換えは h が長いほど Dice の分母が膨らんで 0 に近づく。
// そこで needle と同じ幅（および2倍幅）の窓を1文字ずつすべらせ、
// 窓ごとの Dice の最大値も採る。2倍幅を見るのは、要約元のスパンが
// needle より長いケースを拾うため。
//
// 窓側で部分文字列判定は行わない。needle が窓の部分文字列なら h 全体の
// 部分文字列でもあり、先の MatchScore で既に 1.0 になっている。
//
// needle が正規化後に空（記号や絵文字だけ）なら 0.0。h が空でも 0.0。
//
// 窓が述語末尾（「〜することができました」等）に重なるだけで 0.4〜0.56 に乗るため、
// **内容語がすべて捏造でも高い値が出る**。この指標だけで真偽は判定できない（#1566）。
//
// ponytail: 窓を1文字ずつ総当たりする O(len(h)×len(needle)) の素朴な実装。
// 面接1回の発話（数千文字）× 引用10件程度なら数十msで済む。文書単位に広げるなら
// bigram の出現位置を先にインデックス化して候補窓を絞ること。
func BestMatch(needle string, h Bigrams) float64 {
	n := New(needle)
	if n.Empty() || h.Empty() {
		return 0
	}
	best := h.MatchScore(n)
	// 窓の切り出しにだけ rune 列が要る。構造体に持たせず都度作る
	// （窓ループのコストが支配的で、1回の変換は誤差）。
	hr := []rune(h.norm)
	for _, w := range []int{n.Len(), n.Len() * 2} {
		if best == 1 {
			break
		}
		if w >= len(hr) {
			continue
		}
		for i := 0; i+w <= len(hr); i++ {
			best = max(best, dice(gramSet(hr[i:i+w]), n.bigrams))
		}
	}
	return best
}
