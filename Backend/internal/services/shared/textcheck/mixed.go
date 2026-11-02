package textcheck

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

func findSpans(re *regexp.Regexp, s string) []span {
	var out []span
	for _, m := range re.FindAllStringIndex(s, -1) {
		out = append(out, runeSpan(s, m[0], m[1]))
	}
	return out
}

// MixedWidth は英数字の全角・半角の混在を見る。数字と英字は別々に判定する。
type MixedWidth struct{}

var widthPairs = []struct {
	label      string
	half, full *regexp.Regexp
}{
	{"数字", regexp.MustCompile(`[0-9]+`), regexp.MustCompile(`[０-９]+`)},
	{"英字", regexp.MustCompile(`[A-Za-z]+`), regexp.MustCompile(`[Ａ-Ｚａ-ｚ]+`)},
}

func (MixedWidth) Check(text string) []Finding {
	var out []Finding
	for _, p := range widthPairs {
		half, full := findSpans(p.half, text), findSpans(p.full, text)
		// 同数なら全角側を指摘し、半角にそろえるよう促す
		flagged, isFull := pickMinority(full, half)
		to := "全角"
		if isFull {
			to = "半角"
		}
		for _, sp := range flagged {
			out = append(out, Finding{
				Rule: "mixed_width", Severity: SeverityWarning, Start: sp.start, End: sp.end,
				Message: fmt.Sprintf("%sの全角と半角が混在しています（全角 %d 箇所・半角 %d 箇所）", p.label, len(full), len(half)),
				Fix:     fmt.Sprintf("%sを%sにそろえる", p.label, to),
			})
		}
	}
	sortFindings(out)
	return out
}

// MixedNumeral は算用数字と漢数字の混在を見る。
//
// 数字の直後の1文字（「年」「人」「つ」など）を「数える対象」とみなし、同じ対象が
// 算用数字と漢数字の両方で現れたときだけ指摘する。「三年間」と「3年」は指摘し、
// 「一つ」だけなら指摘しない。前が漢字の漢数字（「統一」「第一」）は熟語の一部として数えない。
type MixedNumeral struct{}

const kanjiNumerals = "〇一二三四五六七八九十百千万"

func isArabicDigit(r rune) bool  { return (r >= '0' && r <= '9') || (r >= '０' && r <= '９') }
func isKanjiNumeral(r rune) bool { return strings.ContainsRune(kanjiNumerals, r) }
func isJapaneseWord(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana)
}

func (MixedNumeral) Check(text string) []Finding {
	rs := []rune(text)
	type forms struct{ arabic, kanji []span }
	byKey := map[rune]*forms{}
	for i := 0; i < len(rs); {
		kanji := isKanjiNumeral(rs[i]) && (i == 0 || !unicode.Is(unicode.Han, rs[i-1]))
		if !kanji && !isArabicDigit(rs[i]) {
			i++
			continue
		}
		j := i
		for j < len(rs) && ((kanji && isKanjiNumeral(rs[j])) || (!kanji && isArabicDigit(rs[j]))) {
			j++
		}
		if j < len(rs) && isJapaneseWord(rs[j]) && !isKanjiNumeral(rs[j]) {
			f := byKey[rs[j]]
			if f == nil {
				f = &forms{}
				byKey[rs[j]] = f
			}
			if kanji {
				f.kanji = append(f.kanji, span{i, j + 1})
			} else {
				f.arabic = append(f.arabic, span{i, j + 1})
			}
		}
		i = j
	}

	keys := make([]rune, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool { return keys[a] < keys[b] })

	var out []Finding
	for _, k := range keys {
		f := byKey[k]
		// 同数なら漢数字側を指摘し、算用数字にそろえるよう促す
		flagged, isKanji := pickMinority(f.kanji, f.arabic)
		to := "漢数字"
		if isKanji {
			to = "算用数字"
		}
		for _, sp := range flagged {
			out = append(out, Finding{
				Rule: "mixed_numeral", Severity: SeverityWarning, Start: sp.start, End: sp.end,
				Message: fmt.Sprintf("「%c」の数が算用数字と漢数字で混在しています（算用数字 %d 箇所・漢数字 %d 箇所）", k, len(f.arabic), len(f.kanji)),
				Fix:     fmt.Sprintf("%sにそろえる", to),
			})
		}
	}
	sortFindings(out)
	return out
}

// MixedEra は和暦と西暦の混在を見る。
type MixedEra struct{}

var (
	warekiRe  = regexp.MustCompile(`(令和|平成|昭和)(元|[0-9０-９]+)年`)
	seirekiRe = regexp.MustCompile(`[0-9０-９]{4}年`)
)

func (MixedEra) Check(text string) []Finding {
	wareki, seireki := findSpans(warekiRe, text), findSpans(seirekiRe, text)
	// 同数なら和暦側を指摘し、西暦にそろえるよう促す
	flagged, isWareki := pickMinority(wareki, seireki)
	to := "和暦"
	if isWareki {
		to = "西暦"
	}
	var out []Finding
	for _, sp := range flagged {
		out = append(out, Finding{
			Rule: "mixed_era", Severity: SeverityWarning, Start: sp.start, End: sp.end,
			Message: fmt.Sprintf("和暦と西暦が混在しています（和暦 %d 箇所・西暦 %d 箇所）", len(wareki), len(seireki)),
			Fix:     fmt.Sprintf("%sにそろえる", to),
		})
	}
	return out
}

// MixedStyle は「です・ます調」と「である調」の混在を見る。
//
// 「。」「！」「？」で終わる文だけを、文末で分類する。体言止め・疑問文など
// どちらとも言えない文は数えない。改行だけで終わる行（箇条書き・見出し）は文とみなさない。
type MixedStyle struct{}

const (
	styleNone = iota
	stylePolite
	stylePlain
)

var politeEndings = []string{"です", "ます", "でした", "ました", "ません", "でしょう", "ましょう", "ください"}

// 常体の文末はひらがな1文字で見る。敬体を先に判定するので、
// 「ました」「でしょう」「ください」がここに落ちることはない。
const plainLastRunes = "だたるいうぬ"

func classifyEnding(s string) int {
	for _, e := range politeEndings {
		if strings.HasSuffix(s, e) {
			return stylePolite
		}
	}
	rs := []rune(s)
	if len(rs) > 0 && strings.ContainsRune(plainLastRunes, rs[len(rs)-1]) {
		return stylePlain
	}
	return styleNone
}

func isSentenceEnd(r rune) bool { return strings.ContainsRune("。！？!?", r) }

func (MixedStyle) Check(text string) []Finding {
	rs := []rune(text)
	var polite, plain []span
	start := 0
	for i, r := range rs {
		if !isSentenceEnd(r) {
			continue
		}
		s := start
		for s < i && unicode.IsSpace(rs[s]) {
			s++
		}
		body := strings.TrimRightFunc(string(rs[s:i]), func(c rune) bool {
			return unicode.IsSpace(c) || strings.ContainsRune("」』）)", c)
		})
		switch classifyEnding(body) {
		case stylePolite:
			polite = append(polite, span{s, i + 1})
		case stylePlain:
			plain = append(plain, span{s, i + 1})
		}
		start = i + 1
	}
	// 同数ならである調側を指摘し、です・ます調にそろえるよう促す
	flagged, isPlain := pickMinority(plain, polite)
	to := "常体（である調）"
	if isPlain {
		to = "敬体（です・ます調）"
	}
	var out []Finding
	for _, sp := range flagged {
		out = append(out, Finding{
			Rule: "mixed_style", Severity: SeverityWarning, Start: sp.start, End: sp.end,
			Message: fmt.Sprintf("です・ます調とである調が混在しています（です・ます %d 文・である %d 文）", len(polite), len(plain)),
			Fix:     fmt.Sprintf("%sにそろえる", to),
		})
	}
	return out
}
