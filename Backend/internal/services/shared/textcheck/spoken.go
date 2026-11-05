package textcheck

import (
	"strings"
	"unicode/utf16"
)

// SpokenLanguage は、書き言葉の文書に混ざった話し言葉・略語を指摘する（#1530）。
//   - すごく・すごい・やっぱり・やっぱ・ちゃんと・いっぱい: warning
//   - 略語の バイト・就活: warning。アルバイト・メガバイトのようにカタカナに続く「バイト」は除く
//   - 文頭の なので、・でも、・だから、: warning。文頭は本文の先頭・改行の後・文末（。！？!?）の後で、
//     行頭の空白と箇条書きの記号は飛ばす
//   - させていただ: 2回目以降を info
//
// 「」『』の中は引用なので見ない（MixedStyle と同じ）。閉じ忘れた引用を次の行に持ち越さないよう、
// 引用の深さは改行で 0 に戻す。
type SpokenLanguage struct{}

func (SpokenLanguage) Name() string { return "spoken_language" }

type spokenWord struct {
	word             string
	fix              string
	abbrev           bool
	notAfterKatakana bool
}

// 長いものを先に並べる（「やっぱり」を「やっぱ」より先に見る）。
var spokenWords = []spokenWord{
	{word: "やっぱり", fix: "やはり"},
	{word: "やっぱ", fix: "やはり"},
	{word: "すごく", fix: "とても・非常に"},
	{word: "すごい", fix: "大きな・優れた"},
	{word: "ちゃんと", fix: "きちんと"},
	{word: "いっぱい", fix: "多くの・たくさんの"},
	{word: "バイト", fix: "アルバイト", abbrev: true, notAfterKatakana: true},
	{word: "就活", fix: "就職活動", abbrev: true},
}

var spokenOpeners = []spokenWord{
	{word: "なので", fix: "そのため、"},
	{word: "でも", fix: "しかし、"},
	{word: "だから", fix: "したがって、"},
}

const spokenHumble = "させていただ"

func (r SpokenLanguage) Check(text string) []Finding {
	rs := []rune(text)
	off := runeUTF16Offsets(rs)
	var out []Finding
	add := func(i, n int, sev, msg, fix string) {
		out = append(out, Finding{Rule: r.Name(), Severity: sev, Start: off[i], End: off[i+n], Message: msg, Fix: fix})
	}
	depth, humble, atStart := 0, 0, true
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch c {
		case '\n':
			depth, atStart = 0, true
			continue
		case '「', '『':
			depth++
			atStart = false
			continue
		case '」', '』':
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth > 0 {
			continue
		}
		if isSentenceEnd(c) {
			atStart = true
			continue
		}
		if atStart && isLineLeadRune(c) {
			continue
		}
		lead := atStart
		atStart = false
		if lead {
			if w, n := matchSpokenOpener(rs, i); n > 0 {
				add(i, n, SeverityWarning, "文頭の「"+w.word+"、」は話し言葉の接続詞です。", w.fix)
				i += n - 1
				continue
			}
		}
		if runesHavePrefix(rs, i, spokenHumble) {
			n := len([]rune(spokenHumble))
			humble++
			if humble >= 2 {
				add(i, n, SeverityInfo, "「させていただく」が繰り返されています。2回目以降は「いたします」などに言い換えると読みやすくなります。", "いたします")
			}
			i += n - 1
			continue
		}
		if w, n := matchSpokenWord(rs, i); n > 0 {
			msg := "「" + w.word + "」は話し言葉です。"
			if w.abbrev {
				msg = "「" + w.word + "」は略語です。書類では略さずに書きます。"
			}
			add(i, n, SeverityWarning, msg, w.fix)
			i += n - 1
		}
	}
	return out
}

func matchSpokenOpener(rs []rune, i int) (spokenWord, int) {
	for _, w := range spokenOpeners {
		n := len([]rune(w.word))
		if runesHavePrefix(rs, i, w.word) && i+n < len(rs) && isCommaRune(rs[i+n]) {
			return w, n + 1
		}
	}
	return spokenWord{}, 0
}

func matchSpokenWord(rs []rune, i int) (spokenWord, int) {
	for _, w := range spokenWords {
		if !runesHavePrefix(rs, i, w.word) {
			continue
		}
		if w.notAfterKatakana && i > 0 && isKatakanaRune(rs[i-1]) {
			continue
		}
		return w, len([]rune(w.word))
	}
	return spokenWord{}, 0
}

func runesHavePrefix(rs []rune, i int, s string) bool {
	p := []rune(s)
	if i+len(p) > len(rs) {
		return false
	}
	for k, c := range p {
		if rs[i+k] != c {
			return false
		}
	}
	return true
}

func isCommaRune(c rune) bool { return c == '、' || c == '，' || c == ',' }

// isLineLeadRune は、行頭・文頭で飛ばす空白と箇条書きの記号。
func isLineLeadRune(c rune) bool {
	return strings.ContainsRune(" \t\r\u3000・•●○■□◆◇▪-－*＊※", c)
}

func isKatakanaRune(c rune) bool { return (c >= 'ァ' && c <= 'ヺ') || c == 'ー' }

// runeUTF16Offsets は、ルーン i の UTF-16 での開始位置を返す（末尾に全体の長さを足す）。
func runeUTF16Offsets(rs []rune) []int {
	off := make([]int, len(rs)+1)
	for i, c := range rs {
		n := utf16.RuneLen(c)
		if n < 1 {
			n = 1
		}
		off[i+1] = off[i] + n
	}
	return off
}
