package masking

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Method は項目の照合の方法。
type Method int

const (
	// MethodDictionary は呼び出し側が渡す既知の名称（Dictionary）と突き合わせる。
	MethodDictionary Method = iota + 1
	// MethodPattern は正規表現（Rule.Pattern）で拾う。
	MethodPattern
)

// Rule は置き換える項目の定義。
type Rule struct {
	// Key は項目の名前。Dictionary のキーにもなる（"name" など）。
	Key string
	// Token は置き換えの記号（"<NAME>" など）。
	Token string
	// Method は照合の方法。
	Method Method
	// Pattern は MethodPattern のときの正規表現。
	Pattern *regexp.Regexp
	// Accept は MethodPattern の一致を採るかの追加の判定（任意）。
	// Go の regexp には前後読みが無いので、前後の文字を見る境界の判定はここで行う。
	// [start, end) は text のバイト位置。
	Accept func(text string, start, end int) bool
}

// Dictionary は項目の Key ごとの既知の名称。氏名は NameTerms で揺れを足してから入れる。
type Dictionary map[string][]string

// minTermRunes は辞書の名称として照合する最小の長さ（正規化後のルーン数）。
// 1文字の名称（姓の「林」など）は、日本語に語の区切りが無いため「森林」「林業」のような
// 普通の語まで壊す。本文を不可逆に失う害のほうが大きいので照合せず、抜き取り確認で拾う。
const minTermRunes = 2

type term struct {
	folded []rune
	rule   int
}

// Masker は項目の定義と辞書から作った置き換えの処理。作ったあとは読み取りだけなので、
// 複数の goroutine から同時に Mask を呼んでよい。
type Masker struct {
	rules []Rule
	// byFirst は正規化した名称を先頭の文字で引く索引。各スライスは長い順（同じ長さなら項目の順）。
	byFirst map[rune][]term
}

// New は項目の定義と辞書から Masker を作る。
// 定義の誤り（Key の重複、記号の空、正規表現の無い MethodPattern など）と、
// 定義に無い Key や辞書で照合しない項目の Key を持つ辞書はエラーにする。
// 辞書のキーの書き間違いを黙って通すと、その項目が置き換わらないまま書き出されるため。
func New(rules []Rule, dict Dictionary) (*Masker, error) {
	keys := make(map[string]int, len(rules))
	for i, r := range rules {
		if r.Key == "" {
			return nil, fmt.Errorf("masking: %d 番目の項目の Key が空です", i)
		}
		if _, dup := keys[r.Key]; dup {
			return nil, fmt.Errorf("masking: 項目 %q が重複しています", r.Key)
		}
		keys[r.Key] = i
		if r.Token == "" {
			return nil, fmt.Errorf("masking: 項目 %q の置き換えの記号が空です", r.Key)
		}
		switch r.Method {
		case MethodDictionary:
		case MethodPattern:
			if r.Pattern == nil {
				return nil, fmt.Errorf("masking: 項目 %q に正規表現がありません", r.Key)
			}
		default:
			return nil, fmt.Errorf("masking: 項目 %q の照合の方法 %d は不明です", r.Key, r.Method)
		}
	}

	m := &Masker{rules: slices.Clone(rules), byFirst: map[rune][]term{}}
	type seenKey struct {
		folded string
		rule   int
	}
	seen := map[seenKey]bool{}
	for key, names := range dict {
		i, ok := keys[key]
		if !ok {
			return nil, fmt.Errorf("masking: 辞書の %q は項目の定義にありません", key)
		}
		if rules[i].Method != MethodDictionary {
			return nil, fmt.Errorf("masking: 項目 %q は辞書で照合しません", key)
		}
		for _, name := range names {
			f := foldTerm(name)
			if len(f) < minTermRunes {
				continue
			}
			k := seenKey{folded: string(f), rule: i}
			if seen[k] {
				continue
			}
			seen[k] = true
			m.byFirst[f[0]] = append(m.byFirst[f[0]], term{folded: f, rule: i})
		}
	}
	for _, ts := range m.byFirst {
		// 長い名称を先に試す。「東京大学大学院」と「東京大学」が同じ位置で一致したとき、
		// 短いほうを採ると「<SCHOOL>大学院」になり、名称の一部が残る。
		slices.SortFunc(ts, func(a, b term) int {
			if c := cmp.Compare(len(b.folded), len(a.folded)); c != 0 {
				return c
			}
			return cmp.Compare(a.rule, b.rule)
		})
	}
	return m, nil
}

type span struct {
	start, end int
	rule       int
}

// Mask は text の中の各項目を置き換えの記号にした文字列を返す。
// 置き換えるのは一致した部分だけで、改行などほかの文字はそのまま残す。
func (m *Masker) Mask(text string) string {
	if text == "" {
		return text
	}

	spans := m.dictionarySpans(text)
	for i, r := range m.rules {
		if r.Method != MethodPattern {
			continue
		}
		for _, loc := range r.Pattern.FindAllStringIndex(text, -1) {
			if loc[0] == loc[1] {
				continue
			}
			if r.Accept != nil && !r.Accept(text, loc[0], loc[1]) {
				continue
			}
			spans = append(spans, span{start: loc[0], end: loc[1], rule: i})
		}
	}
	if len(spans) == 0 {
		return text
	}

	// 重なった一致は、先に始まるもの → 長いもの → 定義で先の項目、の順で採る。
	// 採った一致と重なり、その先まで続く一致は、はみ出した部分も採った記号に含めて隠す（下の cursor の更新）。
	slices.SortFunc(spans, func(a, b span) int {
		if c := cmp.Compare(a.start, b.start); c != 0 {
			return c
		}
		if c := cmp.Compare(b.end-b.start, a.end-a.start); c != 0 {
			return c
		}
		return cmp.Compare(a.rule, b.rule)
	})

	var b strings.Builder
	b.Grow(len(text))
	cursor := 0
	for _, s := range spans {
		if s.start < cursor {
			// 先に採った一致の途中から始まり、その先まで続く一致を捨てると、はみ出した部分が
			// そのまま残って漏れる。はみ出した部分も先の記号に含める（#2232 のレビュー）。
			cursor = max(cursor, s.end)
			continue
		}
		b.WriteString(text[cursor:s.start])
		b.WriteString(m.rules[s.rule].Token)
		cursor = s.end
	}
	b.WriteString(text[cursor:])
	return b.String()
}

// dictionarySpans は辞書の名称に一致する位置を、元の text のバイト位置で返す。
// 各位置では一致する名称のうち最も長いものだけを返す（短いものは重なりの解消で必ず負けるため）。
func (m *Masker) dictionarySpans(text string) []span {
	if len(m.byFirst) == 0 {
		return nil
	}
	f := foldText(text)
	var spans []span
	for i := range f {
		for _, t := range m.byFirst[f[i].r] {
			n := len(t.folded)
			if i+n > len(f) || !matchAt(f, i, t.folded) {
				continue
			}
			start, end := f[i].start, f[i+n-1].end
			if !wordBoundaryOK(text, start, end, t.folded) {
				continue
			}
			spans = append(spans, span{start: start, end: end, rule: t.rule})
			break
		}
	}
	return spans
}

func matchAt(f []foldedRune, i int, want []rune) bool {
	for j, r := range want {
		if f[i+j].r != r {
			return false
		}
	}
	return true
}

// wordBoundaryOK はラテン文字（英数字・アクセント付きの文字）で始まる・終わる名称が、
// 単語の途中に一致していないかを見る。「ABC」が「ABCDEF」の頭に一致して「<COMPANY>DEF」に、
// 「Éric」が「Frédéric」の末尾に一致して「Fréd<NAME>」になるのを防ぐ。
// 日本語には語の区切りが無いので、日本語の文字に挟まれた位置は区切りとみなす。
func wordBoundaryOK(text string, start, end int, folded []rune) bool {
	if isLatinWord(folded[0]) && start > 0 && isLatinWord(foldOne(baseRuneBefore(text[:start]))) {
		return false
	}
	if isLatinWord(folded[len(folded)-1]) && end < len(text) {
		// end は foldText の単位の終わりなので、直後は結合文字でなく基底文字。
		if r, _ := utf8.DecodeRuneInString(text[end:]); isLatinWord(foldOne(r)) {
			return false
		}
	}
	return true
}

// baseRuneBefore は s の末尾の文字を、結合文字なら遡ってその基底文字を返す。
// 「Fre」+U+0301 の直後を、結合文字でなく「e」で判定するため。基底文字が無ければ最後に見た文字を返す。
func baseRuneBefore(s string) rune {
	r, size := utf8.DecodeLastRuneInString(s)
	for isCombining(r) && len(s) > size {
		s = s[:len(s)-size]
		r, size = utf8.DecodeLastRuneInString(s)
	}
	return r
}

// isLatinWord は単語の内部とみなす文字。ASCII の英数字と、Unicode のラテン文字（é など）。
// かな・漢字は含めないので、日本語に挟まれた位置は区切りになる。
func isLatinWord(r rune) bool {
	if r < utf8.RuneSelf {
		return unicode.IsLetter(r) || unicode.IsDigit(r)
	}
	return unicode.Is(unicode.Latin, r)
}

// foldedRune は正規化した1文字と、それが由来する元の文字列のバイト位置。
type foldedRune struct {
	r          rune
	start, end int
}

// isIgnorable は照合のときに無視する文字。氏名の「山田 太郎」「山田　太郎」「山田太郎」、
// 外国の名前の「ジョン・スミス」を同じ名称として扱うため。
// 改行は無視しない。行をまたいで一致させると、置き換えで改行が消えて文章の構造が壊れるため。
func isIgnorable(r rune) bool {
	switch r {
	case ' ', '\t', '\u00A0', '\u3000', '・', '･':
		return true
	// ゼロ幅の文字（ZWSP・ZWNJ・ZWJ・WORD JOINER・BOM）。見えないので、名称の途中に紛れても
	// 照合を切らないよう無視する（「山\u200b田太郎」で姓が漏れないように。#2232 のレビュー）
	case '\u200B', '\u200C', '\u200D', '\u2060', '\uFEFF':
		return true
	}
	return false
}

// isCombining は直前の基底文字とまとめて正規化する文字。結合文字（Mn・Mc・Me）と、
// NFKC で結合用の濁点・半濁点になる半角カナの「ﾞ」「ﾟ」。
func isCombining(r rune) bool {
	return r == '\uFF9E' || r == '\uFF9F' || unicode.Is(unicode.M, r)
}

// foldText は照合用に text を正規化する。基底文字とそれに続く結合文字を1つの単位として
// NFKC（全角英数字→半角、半角カナ→全角、結合文字の合成）と小文字化をかけ、無視する文字を落とす。
// 1文字ずつ正規化すると、合成済みの「José」と結合文字を使った「Jose」+U+0301 や、
// 「ﾀﾞ」と「ダ」のような等価な書き方が一致しないため。
// 単位から出た文字はすべて単位全体のバイト位置を持つので、置き換えが単位の途中で切れることはない。
func foldText(s string) []foldedRune {
	out := make([]foldedRune, 0, len(s))
	for i := 0; i < len(s); {
		// 不正なバイト列は DecodeRuneInString が1バイトずつ進むので、長さは実際のバイト数で取る。
		r, size := utf8.DecodeRuneInString(s[i:])
		if isIgnorable(r) {
			i += size
			continue
		}
		end := i + size
		for end < len(s) {
			next, n := utf8.DecodeRuneInString(s[end:])
			if !isCombining(next) {
				break
			}
			end += n
		}
		for _, fr := range foldUnit(s[i:end]) {
			out = append(out, foldedRune{r: fr, start: i, end: end})
		}
		i = end
	}
	return out
}

func foldTerm(name string) []rune {
	f := foldText(name)
	out := make([]rune, len(f))
	for i, fr := range f {
		out[i] = fr.r
	}
	return out
}

// foldUnit は基底文字と結合文字の1単位（または1文字）を NFKC と小文字化で正規化する。
func foldUnit(u string) []rune {
	if len(u) == 1 && u[0] < utf8.RuneSelf {
		return []rune{unicode.ToLower(rune(u[0]))}
	}
	rs := []rune(norm.NFKC.String(u))
	for i, x := range rs {
		rs[i] = unicode.ToLower(x)
	}
	return rs
}

// foldOne は境界の判定用に1文字を正規化する（全角英字も英字として扱うため）。
func foldOne(r rune) rune {
	if rs := foldUnit(string(r)); len(rs) == 1 {
		return rs[0]
	}
	return r
}

// NameTerms は氏名1つから、辞書に入れる揺れを作る。
// 空白（全角も）か中黒で区切られていれば、姓名の連結・名姓の連結・姓・名を返す。
// 空白の有無と全角半角の揺れは照合のときに吸収するので、ここでは足さない。
// 区切りが無い氏名（「山田太郎」）は姓と名の境目が分からないので、そのまま1つだけ返す。
func NameTerms(fullName string) []string {
	parts := strings.FieldsFunc(fullName, func(r rune) bool {
		return unicode.IsSpace(r) || isIgnorable(r)
	})
	switch len(parts) {
	case 0:
		return nil
	case 1:
		return parts
	}
	reversed := slices.Clone(parts)
	slices.Reverse(reversed)
	terms := []string{strings.Join(parts, ""), strings.Join(reversed, "")}
	return append(terms, parts...)
}
