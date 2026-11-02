package textcheck

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// DateOrder は期間の書き方（「2020年4月〜2021年3月」）で、開始が終了より後になっている矛盾を見る。
// 和暦は西暦に直して比べる。月が片方に無いときは年だけで比べる。
type DateOrder struct{}

const datePattern = `(?:(令和|平成|昭和)(元|[0-9]+)|([0-9]{4}))年(?:\s*([0-9]{1,2})月)?`

var dateRangeRe = regexp.MustCompile(datePattern + `\s*(?:〜|～|~|－|-|ー|から)\s*` + datePattern)

var eraBase = map[string]int{"令和": 2018, "平成": 1988, "昭和": 1925}

func (DateOrder) Check(text string) []Finding {
	s := normalizeDigits(text)
	var out []Finding
	for _, m := range dateRangeRe.FindAllStringSubmatchIndex(s, -1) {
		y1, m1 := yearMonth(s, m, 1)
		y2, m2 := yearMonth(s, m, 5)
		if !(y1 > y2 || (y1 == y2 && m1 > 0 && m2 > 0 && m1 > m2)) {
			continue
		}
		sp := runeSpan(s, m[0], m[1])
		out = append(out, Finding{
			Rule: "date_order", Severity: SeverityCritical, Start: sp.start, End: sp.end,
			Message: fmt.Sprintf("期間の開始（%s）が終了（%s）より後になっています", formatYM(y1, m1), formatYM(y2, m2)),
			Fix:     "開始と終了の年月を確認する",
		})
	}
	return out
}

// yearMonth は datePattern の4グループ（first 番目から）を西暦の年と月に直す。月が無ければ0。
func yearMonth(s string, m []int, first int) (year, month int) {
	group := func(k int) string {
		if m[2*k] < 0 {
			return ""
		}
		return s[m[2*k]:m[2*k+1]]
	}
	if era := group(first); era != "" {
		n := 1
		if v := group(first + 1); v != "元" {
			n, _ = strconv.Atoi(v)
		}
		year = eraBase[era] + n
	} else {
		year, _ = strconv.Atoi(group(first + 2))
	}
	month, _ = strconv.Atoi(group(first + 3))
	return year, month
}

func formatYM(y, m int) string {
	if m == 0 {
		return fmt.Sprintf("%d年", y)
	}
	return fmt.Sprintf("%d年%d月", y, m)
}

// CharLimit は指定字数に対する超過・不足を見る（ES のみ。履歴書には字数指定が無い）。
//
// 数え方と許容範囲は RAG の rag/services/es_review.py（count_es_chars /
// _CHAR_LIMIT_RANGE / _char_limit_bounds）に揃える。RAG は改善文について
// char_limit_satisfied を返すので、Go で別の数え方をすると「収まっています」（改善文）と
// 「字数超過」（原文）が同じ画面で食い違う。RAG 側を変えたらここも変えること。
type CharLimit struct {
	Limit int
	Mode  string // "within" | "around"。未知の値は within 扱い（RAG と同じ）
}

// charLimitRange は (目標下限の比率, 許容上限の比率)。
var charLimitRange = map[string][2]float64{
	"within": {0.85, 1.00},
	"around": {0.90, 1.10},
}

// CountChars は RAG の count_es_chars と同じ数え方で文字数を返す。
// 改行と前後の空白は数えず、それ以外（全角・半角・記号・文中の空白）は1文字と数える。
func CountChars(text string) int {
	t := strings.NewReplacer("\r", "", "\n", "").Replace(text)
	return utf8.RuneCountInString(strings.TrimSpace(t))
}

// bounds は RAG の _char_limit_bounds と同じく、比率を掛けて切り捨てた (下限, 上限) を返す。
func (c CharLimit) bounds() (low, high int) {
	r, ok := charLimitRange[c.Mode]
	if !ok {
		r = charLimitRange["within"]
	}
	return int(float64(c.Limit) * r[0]), int(float64(c.Limit) * r[1])
}

func (c CharLimit) Check(text string) []Finding {
	if c.Limit <= 0 {
		return nil
	}
	n := CountChars(text)
	low, high := c.bounds()
	end := utf8.RuneCountInString(text)
	switch {
	case n > high:
		return []Finding{{
			Rule: "char_limit", Severity: SeverityCritical, Start: 0, End: end,
			Message: fmt.Sprintf("字数が上限を超えています（%d字 / 上限 %d字）", n, high),
			Fix:     fmt.Sprintf("%d字以内に削る", high),
		}}
	case n < low:
		return []Finding{{
			Rule: "char_limit", Severity: SeverityWarning, Start: 0, End: end,
			Message: fmt.Sprintf("字数が少なすぎます（%d字 / 目安 %d字以上）", n, low),
			Fix:     fmt.Sprintf("%d字以上に書き足す", low),
		}}
	}
	return nil
}
