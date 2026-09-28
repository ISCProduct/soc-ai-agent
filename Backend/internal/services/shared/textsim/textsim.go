// Package textsim は日本語の表記ゆれに強いテキスト類似度を提供する。
//
// 形態素解析器を持ち込まずに済むよう、分かち書き不要な「文字bigramのDice係数」を採用している。
// 日本語は空白で単語を区切らないため、空白区切りトークンの一致では一文が丸ごと1トークンになり
// 完全一致以外を拾えない。文字bigramなら語尾や送り仮名の違い・一部省略にも耐えられる。
package textsim

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Normalize は比較用にテキストを正規化する。
//   - NFKC正規化（全角英数字・半角カナなどの統一。例: "Ｗｅｂ" -> "Web"）
//   - 空白・改行・タブの除去
//   - 約物（、。・「」（）!? など）の除去
//   - 小文字化
func Normalize(s string) string {
	s = norm.NFKC.String(strings.TrimSpace(s))
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return -1
		}
		return r
	}, s)
	return strings.ToLower(s)
}

// Text は正規化済み文字列と、その文字bigram集合を保持する。
// 比較対象を何度も突き合わせる場合は New で事前計算して使い回すこと。
type Text struct {
	norm    string
	bigrams map[string]struct{}
}

// New はテキストを正規化し、文字bigramを事前計算する。
func New(s string) Text {
	normalized := Normalize(s)
	return Text{norm: normalized, bigrams: bigrams(normalized)}
}

// Norm は正規化済み文字列を返す。
func (t Text) Norm() string { return t.norm }

// Empty は正規化後に文字が残らなかったかを返す。
func (t Text) Empty() bool { return t.norm == "" }

// Score は2テキストの一致度を 0.0〜1.0 で返す。
// 一方が他方を部分文字列として含む場合は 1.0（そのまま抜粋されたケース）、
// それ以外は文字bigramのDice係数 2|A∩B| / (|A|+|B|) を返す。
func (t Text) Score(other Text) float64 {
	if t.norm == "" || other.norm == "" {
		return 0
	}
	if strings.Contains(t.norm, other.norm) || strings.Contains(other.norm, t.norm) {
		return 1
	}
	if len(t.bigrams) == 0 || len(other.bigrams) == 0 {
		return 0
	}
	small, large := t.bigrams, other.bigrams
	if len(small) > len(large) {
		small, large = large, small
	}
	common := 0
	for gram := range small {
		if _, ok := large[gram]; ok {
			common++
		}
	}
	return 2 * float64(common) / float64(len(t.bigrams)+len(other.bigrams))
}

// Similarity は2つの生テキストの一致度を返す（使い回さない単発比較向け）。
func Similarity(a, b string) float64 {
	return New(a).Score(New(b))
}

// bigrams は正規化済みテキストの隣接2文字の集合を返す。
func bigrams(normalized string) map[string]struct{} {
	runes := []rune(normalized)
	if len(runes) < 2 {
		return nil
	}
	result := make(map[string]struct{}, len(runes)-1)
	for i := range len(runes) - 1 {
		result[string(runes[i:i+2])] = struct{}{}
	}
	return result
}
