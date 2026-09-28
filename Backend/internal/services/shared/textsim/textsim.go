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
//   - 約物・記号（、。・「」（）!? 〜 ~ など）の除去
//   - 小文字化
//
// ただし数字に挟まれた小数点・桁区切りは残す。「1.5倍」と「15倍」は別の記述であり、
// 同一視すると数値が違う箇所に注釈が飛ぶため。
func Normalize(s string) string {
	runes := []rune(norm.NFKC.String(strings.TrimSpace(s)))
	var b strings.Builder
	b.Grow(len(runes))
	for i, r := range runes {
		switch {
		case unicode.IsSpace(r):
			continue
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			// 記号も落とす。波ダッシュ〜(Pd)と全角チルダ～(NFKC後は~/Sm)のように
			// 見た目が同じで分類が違う文字を取りこぼさないため。
			if !isDigitSeparator(runes, i) {
				continue
			}
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

// isDigitSeparator は runes[i] が数字に挟まれた小数点・桁区切りかを判定する。
func isDigitSeparator(runes []rune, i int) bool {
	if runes[i] != '.' && runes[i] != ',' {
		return false
	}
	return i > 0 && i+1 < len(runes) && unicode.IsDigit(runes[i-1]) && unicode.IsDigit(runes[i+1])
}

// Bigrams は正規化済み文字列と、その文字bigram集合を保持する。
// 同じテキストを何度も突き合わせる場合は New で事前計算して使い回すこと。
type Bigrams struct {
	norm    string
	runes   int
	bigrams map[string]struct{}
}

// New はテキストを正規化し、文字bigramを事前計算する。
func New(s string) Bigrams {
	normalized := Normalize(s)
	return Bigrams{
		norm:    normalized,
		runes:   len([]rune(normalized)),
		bigrams: bigramSet(normalized),
	}
}

// Norm は正規化済み文字列を返す。
func (t Bigrams) Norm() string { return t.norm }

// Len は正規化後の文字数を返す（事前計算済み）。
func (t Bigrams) Len() int { return t.runes }

// Empty は正規化後に文字が残らなかったかを返す。
func (t Bigrams) Empty() bool { return t.norm == "" }

// Score は needle が t（本文側）にどれだけ一致するかを 0.0〜1.0 で返す。
// t が needle をそのまま含む場合は 1.0、それ以外は文字bigramのDice係数
// 2|A∩B| / (|A|+|B|) を返す。
//
// 逆向き（t が needle に含まれる）は1.0にしない。OCRの行単位ブロックには
// 「年」「なし」のような短い表ヘッダが必ず混ざり、引用の部分文字列として
// 無条件に最高スコアを取ってしまうため。短いブロックはDice係数で自然に沈み、
// 引用と長く重なるブロックほど高いスコアになる。
func (t Bigrams) Score(needle Bigrams) float64 {
	if t.norm == "" || needle.norm == "" {
		return 0
	}
	if strings.Contains(t.norm, needle.norm) {
		return 1
	}
	if len(t.bigrams) == 0 || len(needle.bigrams) == 0 {
		return 0
	}
	small, large := t.bigrams, needle.bigrams
	if len(small) > len(large) {
		small, large = large, small
	}
	common := 0
	for gram := range small {
		if _, ok := large[gram]; ok {
			common++
		}
	}
	return 2 * float64(common) / float64(len(t.bigrams)+len(needle.bigrams))
}

// Score は haystack に対する needle の一致度を返す（使い回さない単発比較向け）。
// 引数の順序に意味がある非対称な関数。対称な類似度が欲しい場合は別関数を足すこと。
func Score(haystack, needle string) float64 {
	return New(haystack).Score(New(needle))
}

// bigramSet は正規化済みテキストの隣接2文字の集合を返す。
func bigramSet(normalized string) map[string]struct{} {
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
